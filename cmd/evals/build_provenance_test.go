package main

import (
	"runtime/debug"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestVerifyBuildRevisionRejectsMissingDirtyOrStaleBuild(t *testing.T) {
	for _, info := range []*debug.BuildInfo{nil, {}, {Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "old"}, {Key: "vcs.modified", Value: "false"}}}, {Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "current"}, {Key: "vcs.modified", Value: "true"}}}} {
		require.Error(t, verifyBuildRevision(info, "current"))
	}
	require.NoError(t, verifyBuildRevision(&debug.BuildInfo{Settings: []debug.BuildSetting{{Key: "vcs.revision", Value: "current"}, {Key: "vcs.modified", Value: "false"}}}, "current"))
}
