// Copyright (c) 2025 amidgo. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package testenv

import (
	"os"
	"strconv"
	"testing"
)

const envName = "CONTAINERS_DISABLE_TESTING"

func Disabled() bool {
	env := os.Getenv(envName)

	disabled, _ := strconv.ParseBool(env)

	return disabled
}

func SkipDisabled(t *testing.T) {
	if Disabled() {
		t.Skipf("test skipped because %s is SET to TRUE", envName)
	}
}
