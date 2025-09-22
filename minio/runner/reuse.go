// Copyright (c) 2025 amidgo. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package miniorunner

import (
	minioenv "github.com/amidgo/testenv/minio"
)

var reusable = minioenv.NewReusable(RunContainer(nil))

func Reusable() *minioenv.Reusable {
	return reusable
}
