// Copyright (c) 2025 amidgo. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package first_test

import (
	"testing"

	"github.com/amidgo/testenv/internal/testing/reuse"
)

func Test_First(t *testing.T) {
	t.Parallel()

	t.Run("zero user exit", reuse.ReuseDaemon_Zero_User_Exit)
}

func Test_Second(t *testing.T) {
	t.Parallel()

	t.Run("zero user exit", reuse.ReuseDaemon_Zero_User_Exit)
}

func Test_Third(t *testing.T) {
	t.Parallel()

	t.Run("zero user exit", reuse.ReuseDaemon_Zero_User_Exit)
}
