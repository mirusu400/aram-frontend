//go:build !js || !wasm

package frontend

import "path/filepath"

type preparedIssueBundle struct {
	path string
	name string
	data []byte
}

func prepareIssueDebugBundle(
	snapshot debugBundleSnapshot,
	backend Backend,
) (preparedIssueBundle, string, error) {
	path, warning, err := collectDebugBundle(snapshot, backend)
	return preparedIssueBundle{
		path: path,
		name: filepath.Base(path),
	}, warning, err
}
