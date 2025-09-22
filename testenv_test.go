// Copyright (c) 2025 amidgo. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package testenv

import (
	"testing"
)

func Test_Skipped(t *testing.T) {
	t.Setenv(envName, "true")

	SkipDisabled(t)

	t.Fatal("expected test is skipped")
}
