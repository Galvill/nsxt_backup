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

	"github.com/gv/nsxt-fw-backup/internal/applog"
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

	restoreInput               string
	restoreSection             string
	restoreForce               bool
	restoreYes                 bool
	restoreSkipDryRun          bool
	restoreAcceptScopeMismatch bool

	logLevel string
)

func main() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func formatAPIPrefixForMessage(p string) string {
	if strings.TrimSpace(p) == "" {
		return "(none — default / non-tenant Policy path)"
	}
	return p
}

func resolveRestoreAPIPrefix(doc backup.Document) (prefix string, usedBackupScope bool) {
	prefix = apiPrefix()
	if doc.Scope.APIPrefix != "" && prefix == "" {
		return doc.Scope.APIPrefix, true
	}
	return prefix, false
}

func confirmRestoreScopeMismatch(target, recorded string) error {
	fmt.Fprintf(os.Stderr, "warning: restore target API prefix %s differs from the backup scope (%s).\n",
		formatAPIPrefixForMessage(target), formatAPIPrefixForMessage(recorded))
	if restoreAcceptScopeMismatch {
		return nil
	}
	if restoreYes {
		return fmt.Errorf("refusing restore with mismatched org/project scope; pass --accept-scope-mismatch to allow this")
	}
	fmt.Fprint(os.Stderr, "Continue with mismatched scope? [y/N]: ")
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return err
	}
	line = strings.TrimSpace(strings.ToLower(line))
	if line != "y" && line != "yes" {
		return fmt.Errorf("aborted")
	}
	return nil
}

func validateOrgProjectPair() error {
	o := strings.TrimSpace(org)
	p := strings.TrimSpace(project)
	if (o != "") != (p != "") {
		return fmt.Errorf("--org and --project must be supplied together (provide both, or neither)")
	}
	return nil
}

func apiPrefix() string {
	o := strings.TrimSpace(org)
	p := strings.TrimSpace(project)
	if o != "" && p != "" {
		return "orgs/" + url.PathEscape(o) + "/projects/" + url.PathEscape(p)
	}
	return ""
}

func insecureFromEnv() bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv("NSXT_INSECURE_SKIP_TLS_VERIFY")))
	return v == "1" || v == "true" || v == "yes"
}

func makeLogger() (*applog.Logger, error) {
	lvl, err := applog.ParseLevel(logLevel)
	if err != nil {
		return nil, err
	}
	return applog.New(lvl), nil
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
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log verbosity: quiet (errors only), info (progress), or debug (HTTP detail)")

	backupCmd.Flags().StringVarP(&backupOutput, "output", "o", "", "write backup to this JSON file (required)")
	backupCmd.Flags().StringVarP(&backupSection, "section", "s", "", "export only the DFW security policy (section) with this display_name (exact match)")
	backupCmd.Flags().BoolVar(&backupRedact, "redact-host", false, "omit manager host from the backup file")

	restoreCmd.Flags().StringVarP(&restoreInput, "input", "i", "", "read backup from this JSON file (required)")
	restoreCmd.Flags().StringVarP(&restoreSection, "section", "s", "", "restore only this DFW security policy (section) by display_name (exact match; subset of backup)")
	restoreCmd.Flags().BoolVar(&restoreForce, "force", false, "overwrite existing objects on the manager")
	restoreCmd.Flags().BoolVarP(&restoreYes, "yes", "y", false, "do not prompt for confirmation after dry-run")
	restoreCmd.Flags().BoolVar(&restoreSkipDryRun, "skip-dry-run", false, "skip printing the plan preview (requires -y)")
	restoreCmd.Flags().BoolVar(&restoreAcceptScopeMismatch, "accept-scope-mismatch", false, "allow restore when --org/--project target differs from backup scope (required with -y when they differ)")

	rootCmd.AddCommand(backupCmd, restoreCmd)
	backupCmd.MarkFlagRequired("output")
	restoreCmd.MarkFlagRequired("input")
}

func runBackup(_ *cobra.Command, _ []string) error {
	log, err := makeLogger()
	if err != nil {
		return err
	}
	if err := validateOrgProjectPair(); err != nil {
		return err
	}
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
		Log:         log,
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
	log.Infof("wrote %d resources to %s", len(doc.Resources), backupOutput)
	return nil
}

func runRestore(_ *cobra.Command, _ []string) error {
	log, err := makeLogger()
	if err != nil {
		return err
	}
	if restoreSkipDryRun && !restoreYes {
		return fmt.Errorf("--skip-dry-run requires -y")
	}
	if err := validateOrgProjectPair(); err != nil {
		return err
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
		log.Infof("section %q: restoring %d of %d resources from backup", strings.TrimSpace(restoreSection), len(resources), len(doc.Resources))
	}

	prefix, usedBackupScope := resolveRestoreAPIPrefix(doc)
	if usedBackupScope {
		log.Infof("using API prefix from backup scope: %q", prefix)
	}
	recorded := doc.Scope.RecordedAPIPrefix()
	if strings.TrimSpace(prefix) != strings.TrimSpace(recorded) {
		if err := confirmRestoreScopeMismatch(prefix, recorded); err != nil {
			return err
		}
	}

	c, err := newClient()
	if err != nil {
		return err
	}

	steps, err := restore.BuildPlan(c, prefix, resources, restoreForce, log)
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

	if err := restore.Apply(c, prefix, resources, steps, log); err != nil {
		return err
	}
	log.Infof("restore completed")
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
