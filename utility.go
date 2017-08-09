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

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

//setActiveRepoEnvironmentVariables sets environment variables
func setActiveRepoEnvironmentVariables(id int) {
	repo, err := getRepo(id)
	if err != nil {
		msg.setError(err.Error())
	}
	setEnvironmentVariables(repo.EnvVariables...)
	setEnvironmentVariables("RESTIC_PASSWORD=" + repo.Password)
}

//getRepoName returns a name for the id
func getRepoName(id int) string {
	readFromConfig()
	if repo, err := getRepo(id); err == nil {
		return repo.Name
	}
	return ""
}

//getBackupJobName returns a name for the id
func getBackupJobName(id int) string {
	readFromConfig()
	if job, err := getBackupJob(id); err == nil {
		return job.Name
	}
	return ""
}

//getRepo returns the Repo for the id
func getRepo(id int) (*Repo, error) {
	readFromConfig()
	var repo *Repo
	for _, rep := range configData.Repos {
		if rep.ID == id {
			return &rep, nil
		}
	}
	return repo, errors.New("No such Repo")
}

//getBackupJob returns the BackupJob for the id
func getBackupJob(id int) (*BackupJob, error) {
	readFromConfig()
	var job *BackupJob
	for _, jb := range configData.BackupJobs {
		if jb.ID == id {
			return &jb, nil
		}
	}
	return job, errors.New("No such BackupJob")
}

//setEnvironmentVariables accepts slices like []string{"VARNAME=VALUE"}
func setEnvironmentVariables(envVarSlice ...string) {
	readFromConfig()
	for _, variable := range envVarSlice {
		pair := strings.Split(variable, "=")
		os.Setenv(pair[0], pair[1])
	}
}

//openDefaultBrowser opens "localhost/gui" in the default browser
func openDefaultBrowser() {
	switch runtime.GOOS {
	case "windows":
		err := exec.Command("rundll32", "url.dll,FileProtocolHandler", "http://localhost/gui").Run()
		if err != nil {
			exec.Command("cmd", "/c", "start", "http://localhost/gui").Run()
		}
	case "linux":
		exec.Command("xdg-open", "http://localhost/gui").Run()
	case "darwin":
		exec.Command("open", "http://localhost/gui").Run()
	}
}

//formatTime formats time to look like "2006-01-02T03:04"
func formatTime(tm time.Time) string {
	return tm.Format("2006-01-02T15:04")
}

//timeNow retuns the formatted time
func timeNow() string {
	return formatTime(time.Now())
}

//readDirectory returns the file info and the full path
func readDirectory(directory string) []FileInfoWithPath {
	files, err := ioutil.ReadDir(directory)
	if err != nil {
		msg.setError(err.Error())
	}
	fileinfo := make([]FileInfoWithPath, len(files))
	for i := 0; i < len(files); i++ {
		fileinfo[i] = FileInfoWithPath{FileInfo: files[i], Path: directory + "\\" + files[i].Name()}
	}
	return fileinfo
}

//containsFile looks for a path in a slice
func containsFile(jobFiles []string, path string) bool {
	path = filepath.Clean(path)
	for _, file := range jobFiles {
		file = filepath.Clean(file)
		if file == path {
			return true
		}
	}
	return false
}

//deleteBackupJob deletes a BackupJob
func deleteBackupJob(jobID int) {
	readFromConfig()
	deleteScheduledTask(jobID)
	//find the index of the job to delete
	jobIndexToDelete := -1
	for index, job := range configData.BackupJobs {
		if jobID == job.ID {
			jobIndexToDelete = index
			break
		}
	}
	if jobIndexToDelete == -1 {
		return
	}
	//cut the slice
	configData.BackupJobs = append(configData.BackupJobs[:jobIndexToDelete], configData.BackupJobs[jobIndexToDelete+1:]...)
	writeToConfig()
}

//getSnapshotList parses the output from "restic snapshots"
func getSnapshotList(repoID int) []Snapshot {
	setActiveRepoEnvironmentVariables(repoID)
	repo, err := getRepo(repoID)
	if err != nil {
		msg.setError(err.Error())
	}
	snapshotCmd := exec.Command(resticPath, "-r", repo.Location, "snapshots", "--json")
	outputBytes, _ := snapshotCmd.CombinedOutput()
	snapshots := []Snapshot{}
	json.Unmarshal(outputBytes, &snapshots)
	return snapshots
}

//parseTemplate parses the Template from the supplied Path, it has to be made available by "bo-bindata-assetfs" command
func parseTemplate(path string) *template.Template {
	templateFolder := "data/templates/"
	if configData.Settings.Language == "german" {
		templateFolder = "data/templates_german/"
	}
	templateBytes := string(MustAsset(path))
	funcMap := template.FuncMap{
		"base":         filepath.Base,
		"dir":          filepath.Dir,
		"reponame":     getRepoName,
		"join":         strings.Join,
		"formattime":   formatTime,
		"containsfile": containsFile,
		"trim":         trimLongID,
		"timenow":      timeNow,
	}
	tmplate := template.Must(template.New("gui").Funcs(funcMap).Parse(templateBytes))
	tmplate = template.Must(tmplate.Parse(string(MustAsset(templateFolder + "navbar.html"))))
	tmplate = template.Must(tmplate.Parse(string(MustAsset(templateFolder + "imports.html"))))
	return tmplate
}

//getSnapshotList parses the output from "restic snapshots"
func getSnapshotFileList(repoID int, snapshotID string) []string {
	setActiveRepoEnvironmentVariables(repoID)
	repo, err := getRepo(repoID)
	if err != nil {
		msg.setError(err.Error())
	}
	snapshotCmd := exec.Command(resticPath, "-r", repo.Location, "ls", snapshotID)
	outputBytes, _ := snapshotCmd.CombinedOutput()
	files := strings.Split(string(outputBytes), "\n")[1:]
	return files
}

//decrypt uses aes with key to decrypt ciphertext
func decrypt(ciphertext, key []byte) []byte {
	block, err := aes.NewCipher(key)
	if err != nil {
		fmt.Println(err.Error())
		return []byte("")
	}
	if len(ciphertext) < aes.BlockSize {
		fmt.Printf("Text too short (%d)", len(ciphertext))
		return []byte("")
	}
	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]
	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)
	return ciphertext
}

//encrypt uses aes with key to encrypt ciphertext
func encrypt(plaintext, key []byte) []byte {
	block, err := aes.NewCipher(key)
	if err != nil {
		fmt.Println(err.Error())
		return []byte("")
	}
	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		fmt.Println(err.Error())
		return []byte("")
	}
	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], plaintext)
	return ciphertext
}

//initConfig creates a config file at first program start and loads all Repos an BackupJobs
func initConfig() {
	if isAdmin {
		configFilePath = os.Getenv("ProgramData") + "\\eitea\\backup.conf"
	} else {
		configFilePath = os.Getenv("AppData") + "\\eitea\\backup.conf"
	}
	if _, err := os.Stat(configFilePath); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(configFilePath), 0777)
		configData = ConfigData{Settings: Setting{AutomaticPageReload: true, AutomaticScroll: true, OpenBrowser: true, ShowMessages: true, Tips: true}}
		if initial.ID != 0 {
			configData.Repos = append(configData.Repos, initial)
		}
		writeToConfig()
	}
	readFromConfig()
}

//writeToConfig saves the config file
func writeToConfig() {
	bytes, err := json.Marshal(configData)
	if err != nil {
		msg.setError("Error while parsing config")
		return
	}
	ioutil.WriteFile(configFilePath, encrypt(bytes, []byte(defaultPassword)), 0666)
}

//readFromConfig reads the config file
func readFromConfig() {
	content, _ := ioutil.ReadFile(configFilePath)
	json.Unmarshal(decrypt(content, []byte(defaultPassword)), &configData)
}

//validatePassword tests if the password is correct
func validatePassword(p string) {
	readFromConfig()
	if p == configData.Settings.Password {
		passwordCorrect = true
	} else if len(p) > 1 && configData.Settings.Password == "" {
		configData.Settings.Password = p
		passwordCorrect = true
		writeToConfig()
	}
}

//trimLongID keeps only 8 characters and appends ... if characters were removed
func trimLongID(id string) string {
	if len(id) < 8 {
		return id
	}
	return id[:8] + "..."
}

//parseAllTemplates parses all templates either in German or English
func parseAllTemplates() {
	templateFolder := "data/templates/"
	if configData.Settings.Language == "german" {
		templateFolder = "data/templates_german/"
	}
	logTemplate = parseTemplate(templateFolder + "log.html")
	helpTemplate = parseTemplate(templateFolder + "help.html")
	indexTemplate = parseTemplate(templateFolder + "main.html")
	passwordTemplate = parseTemplate(templateFolder + "password.html")
	settingsTemplate = parseTemplate(templateFolder + "settings.html")
	backupJobTemplate = parseTemplate(templateFolder + "backupjob.html")
	snapshotTemplate = parseTemplate(templateFolder + "snapshotlist.html")
	repositoryTemplate = parseTemplate(templateFolder + "repository.html")
	filebrowserTemplate = parseTemplate(templateFolder + "filebrowser.html")
	newBackupJobTemplate = parseTemplate(templateFolder + "newbackupjob.html")
	snapshotFileTemplate = parseTemplate(templateFolder + "snapshotfile.html")
	editBackupJobTemplate = parseTemplate(templateFolder + "editbackupjob.html")
	newRepositoryTemplate = parseTemplate(templateFolder + "newrepository.html")
	directoryListTemplate = parseTemplate(templateFolder + "directorylist.html")
	editRepositoryTemplate = parseTemplate(templateFolder + "editrepository.html")
}

//appendLog adds a log entry to a Backup Job
func appendLog(jobID int, success bool, text string) {
	readFromConfig()
	jobIndex := -1
	for index, job := range configData.BackupJobs {
		if jobID == job.ID {
			jobIndex = index
			break
		}
	}
	if jobIndex == -1 {
		return
	}
	configData.BackupJobs[jobIndex].LatestRunSuccessful = success
	configData.BackupJobs[jobIndex].Logs = append(configData.BackupJobs[jobIndex].Logs, Log{Success: success, Time: time.Now(), Text: text})
	writeToConfig()
}

//exitTimer exits the program when not used
func exitTimer() {
	shutdownTime = time.Now().Add(time.Minute)
	for t := range time.NewTicker(time.Second).C {
		if t.After(shutdownTime) {
			switch configData.Settings.Language {
			case "english":
				fmt.Println("Detected inactivity, exiting in 30 Seconds")
			case "german":
				fmt.Println("Tab geschlossen, Beenden in 30 Sekunden")
			}
			time.Sleep(30 * time.Second)
			if time.Now().After(shutdownTime) {
				os.Exit(0)
			}
		}
	}
}

//modifyFileList deletes empty strings
func modifyFileList(s []string) []string {
	var r []string
	for _, str := range s {
		if str != "" {
			r = append(r, str)
		}
	}
	if len(r) < 1 {
		r = append(r, "")
	}
	return r
}

func checkForAdmin() bool {
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	if err != nil {
		return false
	}
	return true
}
