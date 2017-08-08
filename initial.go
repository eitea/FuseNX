package main

var initial = Repo{
	Name:         "Backup",
	Type:         "s3",
	Location:     "s3:s3.amazonaws.com/bucket_name",
	EnvVariables: []string{"AWS_ACCESS_KEY_ID=MY_ACCESS_KEY", "AWS_SECRET_ACCESS_KEY=MY_SECRET_ACCESS_KEY"},
	ID:           0, //ignored when id == 0
	Password:     "password",
}
