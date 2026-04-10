package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/gv/nsxt-fw-backup/internal/backup"
	"github.com/gv/nsxt-fw-backup/internal/dfw"
	"github.com/gv/nsxt-fw-backup/internal/nsx"
	"github.com/gv/nsxt-fw-backup/internal/restore"
)

var (
	host     string
	domain   string
	org      string
	project  string
	insecure bool

	backupOutput  string
	backupSection string
	backupRedact  bool

	restoreInput      string
	restoreSection    string
	restoreForce      bool
	restoreYes        bool
	restoreSkipDryRun bool
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func apiPrefix() string {
	o := strings.TrimSpace(org)
	p := strings.TrimSpace(project)
	if o != "" && p != "" {
		return "orgs/" + url.PathEscape(o) + "/projects/" + url.PathEscape(p)
	}
	if o != "" || p != "" {
		fmt.Fprintln(os.Stderr, "warning: both --org and --project are required for multi-tenant paths; ignoring partial values")
	}
	return ""
}

func insecureFromEnv() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("NSXT_INSECURE_SKIP_TLS_VERIFY")))
	return v == "1" || v == "true" || v == "yes"
}

func newClient() (*nsx.Client, error) {
	auth, err := nsx.AuthFromEnv()
	if err != nil {
		return nil, err
	}
	skip := insecure || insecureFromEnv()
	return nsx.NewClient(nsx.Options{
		Host:               host,
		InsecureSkipVerify: skip,
	}, auth)
}

const envVarsHelp = `Required environment (unless overridden by flags where noted):

  NSXT_MANAGER_HOST or NSXT_HOST   Policy API manager hostname or URL (required if --host is unset)

  Either basic auth or bearer (not both):
    NSXT_USERNAME and NSXT_PASSWORD
    NSXT_BEARER_TOKEN or NSXT_API_KEY

Optional:

  NSXT_INSECURE_SKIP_TLS_VERIFY   If set to 1, true, or yes, skip TLS certificate verification
                                  (same effect as --insecure-skip-tls-verify).`

var rootCmd = &cobra.Command{
	Use:   "nsxt-fw-backup",
	Short: "Backup and restore NSX-T distributed firewall Policy API configuration",
	Long:  "Backup and restore NSX-T distributed firewall Policy API configuration.\n\n" + envVarsHelp,
}

var backupCmd = &cobra.Command{
	Use:   "backup",
	Short: "Export DFW security policies and referenced objects to JSON",
	Long:  envVarsHelp,
	RunE:  runBackup,
}

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Restore DFW configuration from a JSON backup",
	Long:  envVarsHelp,
	RunE:  runRestore,
}

func init() {
	rootCmd.PersistentFlags().StringVar(&host, "host", "", "NSX-T manager host or URL (or NSXT_MANAGER_HOST / NSXT_HOST)")
	rootCmd.PersistentFlags().StringVar(&domain, "domain", "default", "DFW domain id")
	rootCmd.PersistentFlags().StringVar(&org, "org", "", "Organization id for multi-tenant Policy API paths")
	rootCmd.PersistentFlags().StringVar(&project, "project", "", "Project id for multi-tenant Policy API paths")
	rootCmd.PersistentFlags().BoolVar(&insecure, "insecure-skip-tls-verify", false, "Skip TLS verification (or NSXT_INSECURE_SKIP_TLS_VERIFY=true)")

	backupCmd.Flags().StringVarP(&backupOutput, "output", "o", "", "write backup to this JSON file (required)")
	backupCmd.Flags().StringVarP(&backupSection, "section", "s", "", "export only the DFW security policy (section) with this display_name (exact match)")
	backupCmd.Flags().BoolVar(&backupRedact, "redact-host", false, "omit manager host from the backup file")

	restoreCmd.Flags().StringVarP(&restoreInput, "input", "i", "", "read backup from this JSON file (required)")
	restoreCmd.Flags().StringVarP(&restoreSection, "section", "s", "", "restore only this DFW security policy (section) by display_name (exact match; subset of backup)")
	restoreCmd.Flags().BoolVar(&restoreForce, "force", false, "overwrite existing objects on the manager")
	restoreCmd.Flags().BoolVarP(&restoreYes, "yes", "y", false, "do not prompt for confirmation after dry-run")
	restoreCmd.Flags().BoolVar(&restoreSkipDryRun, "skip-dry-run", false, "skip printing the plan preview (requires -y)")

	rootCmd.AddCommand(backupCmd, restoreCmd)
	backupCmd.MarkFlagRequired("output")
	restoreCmd.MarkFlagRequired("input")
}

func runBackup(_ *cobra.Command, _ []string) error {
	c, err := newClient()
	if err != nil {
		return err
	}
	prefix := apiPrefix()
	managerHost := host
	if managerHost == "" {
		managerHost = strings.TrimSpace(os.Getenv("NSXT_MANAGER_HOST"))
	}
	if managerHost == "" {
		managerHost = strings.TrimSpace(os.Getenv("NSXT_HOST"))
	}

	doc, err := dfw.Export(context.Background(), dfw.ExportOptions{
		Client:      c,
		APIPrefix:   prefix,
		Domain:      domain,
		Section:     backupSection,
		RedactHost:  backupRedact,
		ManagerHost: managerHost,
		Org:         org,
		Project:     project,
	})
	if err != nil {
		return err
	}

	f, err := os.Create(backupOutput)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(doc); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %d resources to %s\n", len(doc.Resources), backupOutput)
	return nil
}

func runRestore(_ *cobra.Command, _ []string) error {
	if restoreSkipDryRun && !restoreYes {
		return fmt.Errorf("--skip-dry-run requires -y")
	}

	data, err := os.ReadFile(restoreInput)
	if err != nil {
		return err
	}
	var doc backup.Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return err
	}
	if doc.FormatVersion != backup.FormatVersion {
		return fmt.Errorf("unsupported backup format_version %d (expected %d)", doc.FormatVersion, backup.FormatVersion)
	}
	if len(doc.Resources) == 0 {
		return fmt.Errorf("backup contains no resources")
	}

	resources := doc.Resources
	if strings.TrimSpace(restoreSection) != "" {
		var err error
		resources, err = restore.FilterResourcesForSection(doc.Resources, restoreSection)
		if err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "section %q: restoring %d of %d resources from backup\n", strings.TrimSpace(restoreSection), len(resources), len(doc.Resources))
	}

	c, err := newClient()
	if err != nil {
		return err
	}
	prefix := apiPrefix()
	if doc.Scope.APIPrefix != "" && prefix == "" {
		prefix = doc.Scope.APIPrefix
		fmt.Fprintf(os.Stderr, "using api prefix from backup scope: %q\n", prefix)
	}

	steps, err := restore.BuildPlan(c, prefix, resources, restoreForce)
	if err != nil {
		return err
	}

	if !restoreSkipDryRun {
		printPlan(steps)
	}

	if !restoreYes {
		fmt.Fprint(os.Stderr, "Proceed with restore? [y/N]: ")
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			return err
		}
		line = strings.TrimSpace(strings.ToLower(line))
		if line != "y" && line != "yes" {
			return fmt.Errorf("aborted")
		}
	}

	if err := restore.Apply(c, prefix, resources, steps); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "restore completed")
	return nil
}

func printPlan(steps []restore.Step) {
	w := tabwriter.NewWriter(os.Stderr, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ACTION\tKIND\tDISPLAY NAME\tPATH\tDETAIL")
	for _, s := range steps {
		dn := s.DisplayName
		if dn == "" {
			dn = "-"
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", s.Action.String(), s.Kind, dn, s.Path, s.Detail)
	}
	_ = w.Flush()
}
