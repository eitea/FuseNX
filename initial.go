// FuseNX - BackupGUI
// Copyright (C) 2017 Eitea

// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

//initial can set one persistent Repository which has to be initialized before
var initial = Repo{
	Name:         "Backup",
	Type:         "s3",
	Location:     "s3:s3.amazonaws.com/bucket_name",
	EnvVariables: []string{"AWS_ACCESS_KEY_ID=MY_ACCESS_KEY", "AWS_SECRET_ACCESS_KEY=MY_SECRET_ACCESS_KEY"},
	ID:           0, //ignored when id == 0
	Password:     "password",
}

// func init() {
// 	passwordCorrect = true
// }
