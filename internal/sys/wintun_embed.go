//go:build embedwintun

package sys

import _ "embed"

//go:embed wintun.dll
var embeddedWintun []byte
