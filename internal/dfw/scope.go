package dfw

import (
	"fmt"

	"github.com/gv/nsxt-fw-backup/internal/nsx"
)

func statusOK(status int) bool {
	return status >= 200 && status < 300
}

// DecideFetchReferencedLeafForBackup returns whether to enqueue and fetch a referenced leaf when
// backing up under a non-empty tenant API prefix. rootStatus is only used when tenantStatus == 404.
func DecideFetchReferencedLeafForBackup(tenantStatus, rootStatus int) (fetch bool, err error) {
	if statusOK(tenantStatus) {
		return true, nil
	}
	if tenantStatus != 404 {
		return false, fmt.Errorf("unexpected GET under project: status %d", tenantStatus)
	}
	if statusOK(rootStatus) {
		return false, nil
	}
	if rootStatus == 404 {
		return false, fmt.Errorf("referenced object not found under project or default Policy path (404)")
	}
	return false, fmt.Errorf("unexpected GET at default Policy path: status %d", rootStatus)
}

// ShouldSkipRestoreCreateAtTenant returns true when the object is missing under the tenant prefix but
// exists at the default Policy root — it must not be CREATE/UPDATE'd into the project.
func ShouldSkipRestoreCreateAtTenant(tenantStatus, rootStatus int) bool {
	return tenantStatus == 404 && statusOK(rootStatus)
}

// ResolveReferencedLeafForTenantBackup performs GET under tenantPrefix and, if 404, GET with empty prefix.
// When fetch is true, prefetchBody is the successful tenant GET body (caller may store it to avoid a duplicate GET).
// tenantPrefix must be non-empty.
func ResolveReferencedLeafForTenantBackup(c *nsx.Client, tenantPrefix, rel string) (fetch bool, prefetchBody []byte, err error) {
	bodyT, stT, err := c.Get(tenantPrefix, rel)
	if err != nil {
		return false, nil, err
	}
	if statusOK(stT) {
		return true, bodyT, nil
	}
	if stT != 404 {
		if err := nsx.DecodeAPIError(stT, bodyT); err != nil {
			return false, nil, fmt.Errorf("GET under project: %w", err)
		}
		return false, nil, fmt.Errorf("GET under project: unexpected status %d", stT)
	}

	_, stR, err := c.Get("", rel)
	if err != nil {
		return false, nil, err
	}
	if _, err := DecideFetchReferencedLeafForBackup(stT, stR); err != nil {
		return false, nil, err
	}
	// tenant 404 + root 2xx → skip; tenant 404 + root 404 → Decide returned error above
	return false, nil, nil
}
