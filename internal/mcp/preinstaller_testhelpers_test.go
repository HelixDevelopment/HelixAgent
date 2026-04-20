package mcp

// mutatePackageStatusForTest applies mutate to a copy of the stored
// PackageStatus and commits it back atomically via safe.Store.Update.
// Tests use this helper in place of the pre-CONST-029
// `preinstaller.mu.Lock(); preinstaller.statuses[name].Field = v;
// preinstaller.mu.Unlock()` idiom.
func mutatePackageStatusForTest(p *MCPPreinstaller, name string, mutate func(*PackageStatus)) {
	p.statuses.Update(name, func(cur *PackageStatus, present bool) (*PackageStatus, bool) {
		if !present {
			return nil, false
		}
		cp := *cur
		mutate(&cp)
		return &cp, true
	})
}
