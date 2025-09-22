// Copyright (c) 2025 amidgo. All rights reserved.
// Use of this source code is governed by the MIT license that can be found in the LICENSE file.

package postgresrunner

import pgenv "github.com/amidgo/testenv/postgres"

var reusable = pgenv.NewReusable(RunContainer(nil))

func Reusable() *pgenv.Reusable {
	return reusable
}
