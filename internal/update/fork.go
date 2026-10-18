package update

import "strings"

// forkVersionMarker tags builds of this fork, e.g. "4.2.0-cmx.1.0.0".
const forkVersionMarker = "-cmx."

// IsForkBuild reports whether version identifies a build of this fork. Fork
// builds are never offered upstream releases, which would replace them.
func IsForkBuild(version string) bool {
	return strings.Contains(version, forkVersionMarker)
}
