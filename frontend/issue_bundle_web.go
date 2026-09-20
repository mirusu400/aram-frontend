//go:build js && wasm

package frontend

type preparedIssueBundle struct {
	path string
	name string
	data []byte
}

func prepareIssueDebugBundle(
	snapshot debugBundleSnapshot,
	backend Backend,
) (preparedIssueBundle, string, error) {
	name, data, warning, err := collectDebugBundleData(snapshot, backend)
	return preparedIssueBundle{name: name, data: data}, warning, err
}
