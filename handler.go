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
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

//exitHandler exits the program
func exitHandler(w http.ResponseWriter, r *http.Request) {
	if configData.Settings.Language == "german" {
		w.Write([]byte("Sie können dieses Fenster jetzt schließen."))
	} else {
		w.Write([]byte("You can close this Tab now"))
	}
	writeToConfig()
	go os.Exit(0)
}

//helpHandler displays a help page
func helpHandler(w http.ResponseWriter, r *http.Request) {
	data := &GuiData{Message: msg.get(), Config: configData, Version: version}
	helpTemplate.Execute(w, data)
}

//repositoryHandler lists all repositories
func repositoryHandler(w http.ResponseWriter, r *http.Request) {
	if !passwordCorrect {
		http.Redirect(w, r, "/pw", http.StatusSeeOther)
		return
	}
	readFromConfig()
	if len(configData.Repos) == 0 && msg.SeenByUser == true {
		if configData.Settings.Language == "german" {
			msg.set("Konnte keine Archive finden")
		} else {
			msg.set("Couldn't find any Repositories")
		}
	}
	data := &GuiData{Config: configData, Message: msg.get()}
	repositoryTemplate.Execute(w, data)
}

//editRepositoryHandler handles form and input for editing Repos
func editRepositoryHandler(w http.ResponseWriter, r *http.Request) {
	if !passwordCorrect {
		http.Redirect(w, r, "/pw", http.StatusSeeOther)
		return
	}
	r.ParseForm()
	repoID, err := strconv.Atoi(r.Form["repoid"][0])
	if err != nil {
		return
	}
	if r.Form["edit"] == nil {
		readFromConfig()
		repo, err := getRepo(repoID)
		if err != nil {
			return
		}
		data := &GuiData{CurrentRepo: *repo, Config: configData, Message: msg.get()}
		editRepositoryTemplate.Execute(w, data)
	} else {
		if configData.Settings.Language == "german" {
			msg.setError("Konnte Archiv nicht bearbeiten")
		} else {
			msg.setError("Couldn't edit Repository")
		}

		defer http.Redirect(w, r, "/repository", http.StatusSeeOther)
		readFromConfig()
		name := r.Form["name"][0]
		if len(name) == 0 {
			return
		}
		repoIndexToEdit := -1
		for index, repo := range configData.Repos {
			if repoID == repo.ID {
				repoIndexToEdit = index
				break
			}
		}
		if repoIndexToEdit == -1 {
			return
		}
		configData.Repos[repoIndexToEdit].Name = name
		writeToConfig()
		if configData.Settings.Language == "german" {
			msg.setSuccess("Archiv bearbeitet")
		} else {
			msg.setSuccess("Edited Repository")
		}

	}
}

//newRepositoryHandler displays the form for a new repository
func newRepositoryHandler(w http.ResponseWriter, r *http.Request) {
	if !passwordCorrect {
		http.Redirect(w, r, "/pw", http.StatusSeeOther)
		return
	}
	r.ParseForm()
	if r.Form["create"] == nil {
		var data *GuiData
		if r.Form["import"] == nil {
			data = &GuiData{Message: msg.get(), Config: configData}
		} else {
			data = &GuiData{Message: msg.get(), Config: configData, Import: true}
		}
		newRepositoryTemplate.Execute(w, data)
	} else {
		if r.Form["create"][0] == "location" {
			readFromConfig()
			repoID, err := strconv.Atoi(r.Form["repoid"][0])
			if err != nil {
				return
			}
			repoIndexToEdit := -1
			for index, repo := range configData.Repos {
				if repoID == repo.ID {
					repoIndexToEdit = index
					break
				}
			}
			if repoIndexToEdit == -1 {
				return
			}
			configData.Repos[repoIndexToEdit].Location = r.Form["location"][0]
			writeToConfig()
			setActiveRepoEnvironmentVariables(repoID)
			initCmd := exec.Command(resticPath, "-r", configData.Repos[repoIndexToEdit].Location, "init")
			err = initCmd.Run()
			if err != nil {
				msg.setError(err.Error())
				configData.Repos[repoIndexToEdit].Location = ""
			}
			writeToConfig()
			http.Redirect(w, r, "/repository", http.StatusSeeOther)
			return
		}
		if configData.Settings.Language == "german" {
			msg.setError("Konnte kein Archiv erstellen")
		} else {
			msg.setError("Couldn't create Repository")
		}

		name := r.Form["name"][0]
		rType := r.Form["type"][0] //local, sftp, rest, s3, ninio, swift, b2
		var location string
		locationChooser := false
		if r.Form["location"] != nil && r.Form["location"][0] != "" {
			location = r.Form["location"][0]
		} else {
			locationChooser = true
		}
		password := r.Form["password"][0]
		envVar := r.Form["envvar"][0] //e.g. "VAR=VALUE,OTHERVAR=VALUE"
		id := rand.Int()
		for getRepoName(id) != "" || id == 1 {
			id = rand.Int()
		}
		if !locationChooser {
			defer http.Redirect(w, r, "/repository", http.StatusSeeOther)
		} else {
			defer http.Redirect(w, r, "/filebrowser?repoid="+strconv.Itoa(id), http.StatusSeeOther)
		}
		if len(name) == 0 || len(rType) == 0 {
			return
		}
		var envVarSlice []string
		if len(envVar) > 3 {
			if valid, _ := regexp.MatchString("([_a-zA-Z]+=.+,?)+", envVar); !valid {
				return
			}
			envVarSlice = strings.Split(envVar, ",")
			setEnvironmentVariables(envVarSlice...) // for backends that need environment variables
		}
		if locationChooser {
			readFromConfig()
			configData.Repos = append(configData.Repos, Repo{Name: name, Type: rType, Location: location, EnvVariables: envVarSlice, ID: id, Password: password})
			writeToConfig()
			return
		}
		setEnvironmentVariables("RESTIC_PASSWORD=" + password)
		initCmd := exec.Command(resticPath, "-r", location, "init")
		if err := initCmd.Run(); err == nil || r.Form["import"] != nil { //only add repo if there is no error initializing it or if importing existing repository
			readFromConfig()
			configData.Repos = append(configData.Repos, Repo{Name: name, Type: rType, Location: location, EnvVariables: envVarSlice, ID: id, Password: password})
			writeToConfig()
			if configData.Settings.Language == "german" {
				msg.setSuccess("Archiv erstellt")
			} else {
				msg.setSuccess("Created Repository")
			}
		} else {
			msg.setError(err.Error())
		}
	}
}

//backupJobHandler displays all backupJobs
func backupJobHandler(w http.ResponseWriter, r *http.Request) {
	if !passwordCorrect {
		http.Redirect(w, r, "/pw", http.StatusSeeOther)
		return
	}
	readFromConfig()
	if len(configData.BackupJobs) == 0 && msg.SeenByUser == true {
		if configData.Settings.Language == "german" {
			msg.set("Konnte keine Backup Aufgaben finden")
		} else {
			msg.set("Couldn't find any Backup Jobs")
		}

	}
	data := &GuiData{Config: configData, Message: msg.get()}
	backupJobTemplate.Execute(w, data)
}

//snapshotListHandler displays a snapshot list for one Repo
func snapshotListHandler(w http.ResponseWriter, r *http.Request) {
	if !passwordCorrect {
		http.Redirect(w, r, "/pw", http.StatusSeeOther)
		return
	}
	r.ParseForm()
	repoID, err := strconv.Atoi(r.Form["id"][0])
	if err != nil {
		return
	}
	snapshots := getSnapshotList(repoID)
	if len(snapshots) == 0 && msg.SeenByUser == true {
		if configData.Settings.Language == "german" {
			msg.setError("Konnte keine Snapshots finden")
		} else {
			msg.setError("Couldn't find any Snapshots")
		}

	}
	repo, err := getRepo(repoID)
	if err != nil {
		if configData.Settings.Language == "german" {
			msg.setError("Konnte Archiv nicht finden")
		} else {
			msg.setError("Couldn't find Repo")
		}
	}
	data := &GuiData{Snapshots: snapshots, CurrentRepo: *repo, Message: msg.get(), Config: configData}
	snapshotTemplate.Execute(w, data)
}

//executeBackupHandler creates a new backup for the BackupJob
func executeBackupHandler(w http.ResponseWriter, r *http.Request) {
	if !passwordCorrect {
		http.Redirect(w, r, "/pw", http.StatusSeeOther)
		return
	}
	readFromConfig()
	defer http.Redirect(w, r, "/backupjob", http.StatusSeeOther)
	r.ParseForm()
	jobID, err := strconv.Atoi(r.Form["jobid"][0])
	if err != nil {
		return
	}
	backupJob, err := getBackupJob(jobID)
	if err != nil {
		return
	}
	repo, err := getRepo(backupJob.RepoID)
	if err != nil {
		return
	}
	setActiveRepoEnvironmentVariables(repo.ID)
	backupCmd := exec.Command(resticPath, "-r", repo.Location, "backup")
	backupCmd.Args = append(backupCmd.Args, backupJob.Files...)
	go func() {
		output, err := backupCmd.Output()
		if err != nil {
			appendLog(jobID, false, "[Manual] Error: "+string(output)+" ("+err.Error()+")")
		} else {
			appendLog(jobID, true, "[Manual] Output: "+string(output))
		}
	}()
	if configData.Settings.Language == "german" {
		msg.set("Backup Aufgabe gestartet")
	} else {
		msg.set("Backup Job started")
	}
}

//modifyTagHandler adds and removes tags
func modifyTagHandler(w http.ResponseWriter, r *http.Request) {
	if !passwordCorrect {
		http.Redirect(w, r, "/pw", http.StatusSeeOther)
		return
	}
	r.ParseForm()
	readFromConfig()
	operation := r.Form["operation"][0] //operation should either be "add" or "remove"
	tag := r.Form["tag"][0]
	snapshotID := r.Form["snapshotid"][0]
	repoID, err := strconv.Atoi(r.Form["repoid"][0])
	tag = strings.Replace(tag, " ", "_", -1)
	defer http.Redirect(w, r, "/snapshot?id="+strconv.Itoa(repoID), http.StatusSeeOther)
	if len(operation) == 0 || len(tag) == 0 || tag == "null" || len(snapshotID) == 0 || err != nil {
		return
	}
	repo, err := getRepo(repoID)
	if err != nil {
		return
	}
	tagCmd := exec.Command(resticPath, "-r", repo.Location, "tag", "--"+operation, tag, snapshotID)
	tagCmd.Run()
}

//deleteSnapshotHandler deletes a single Snapshot (only on local filesystem)
func deleteSnapshotHandler(w http.ResponseWriter, r *http.Request) {
	if !passwordCorrect {
		http.Redirect(w, r, "/pw", http.StatusSeeOther)
		return
	}
	r.ParseForm()
	snapshotID := r.Form["snapshotid"][0]
	repoID, err := strconv.Atoi(r.Form["repoid"][0])
	defer http.Redirect(w, r, "/snapshot?id="+strconv.Itoa(repoID), http.StatusSeeOther)
	if len(snapshotID) == 0 || err != nil {
		return
	}
	repo, err := getRepo(repoID)
	if err != nil {
		return
	}
	setActiveRepoEnvironmentVariables(repo.ID)
	exec.Command(resticPath, "-r", repo.Location, "forget", snapshotID, "--prune").Run()
}

//deleteBackupJobHandler deletes a BackupJob
func deleteBackupJobHandler(w http.ResponseWriter, r *http.Request) {
	if !passwordCorrect {
		http.Redirect(w, r, "/pw", http.StatusSeeOther)
		return
	}
	defer http.Redirect(w, r, "/backupjob", http.StatusSeeOther)
	r.ParseForm()
	jobID, err := strconv.Atoi(r.Form["jobid"][0])
	if err != nil {
		return
	}
	deleteBackupJob(jobID)
}

//deleteRepositoryHandler deletes a Repo and its BackupJobs
func deleteRepositoryHandler(w http.ResponseWriter, r *http.Request) {
	if !passwordCorrect {
		http.Redirect(w, r, "/pw", http.StatusSeeOther)
		return
	}
	if configData.Settings.Language == "german" {
		msg.setError("Konnte Archiv nicht löschen")
	} else {
		msg.setError("Couldn't delete Repository")
	}

	defer http.Redirect(w, r, "/repository", http.StatusSeeOther)
	r.ParseForm()
	repoID, err := strconv.Atoi(r.Form["repoid"][0])
	if err != nil {
		return
	}
	if repoID == initial.ID {
		return
	}
	readFromConfig()
	//find the index of the job to delete
	repoIndexToDelete := -1
	for index, repo := range configData.Repos {
		if repoID == repo.ID {
			repoIndexToDelete = index
			break
		}
	}
	if repoIndexToDelete == -1 {
		return
	}
	//delete all BackupJobs for this Repo
	for _, job := range configData.BackupJobs {
		if job.RepoID == repoID {
			deleteBackupJob(job.ID)
		}
	}
	//cut the slice
	configData.Repos = append(configData.Repos[:repoIndexToDelete], configData.Repos[repoIndexToDelete+1:]...)
	writeToConfig()
	if configData.Settings.Language == "german" {
		msg.setSuccess("Archiv gelöscht")
	} else {
		msg.setSuccess("Deleted Repository")
	}

}

//indexHandler handles the start page
func indexHandler(w http.ResponseWriter, r *http.Request) {
	if !passwordCorrect {
		http.Redirect(w, r, "/pw", http.StatusSeeOther)
		return
	}
	data := &GuiData{Message: msg.get(), Config: configData}
	indexTemplate.Execute(w, data)
}

//listSnapshotFileHandler lists all files in a snapshot
func listSnapshotFileHandler(w http.ResponseWriter, r *http.Request) {
	if !passwordCorrect {
		http.Redirect(w, r, "/pw", http.StatusSeeOther)
		return
	}
	r.ParseForm()
	repoID, err := strconv.Atoi(r.Form["repoid"][0])
	snapshotID := r.Form["snapshotid"][0]
	if err != nil {
		return
	}
	files := getSnapshotFileList(repoID, snapshotID)
	if len(files) == 0 && msg.SeenByUser == true {
		if configData.Settings.Language == "german" {
			msg.setError("Keine Dateien gefunden")
		} else {
			msg.setError("Couldn't find any Files")
		}

	}
	repo, err := getRepo(repoID)
	if err != nil {
		if configData.Settings.Language == "german" {
			msg.setError("Archiv nicht gefunden")
		} else {
			msg.setError("Couldn't find Repo")
		}
	}
	data := &GuiData{SnapshotFiles: files, CurrentRepo: *repo, CurrentSnapshotID: snapshotID, Message: msg.get(), Config: configData}
	snapshotFileTemplate.Execute(w, data)
}

//restoreSnapshotHandler restores one snapshot to the specified target
func restoreSnapshotHandler(w http.ResponseWriter, r *http.Request) {
	if !passwordCorrect {
		http.Redirect(w, r, "/pw", http.StatusSeeOther)
		return
	}
	r.ParseForm()
	if configData.Settings.Language == "german" {
		msg.setError("Snapshot konnte nicht wiederhergestellt werden")
	} else {
		msg.setError("Couldn't restore Snapshot")
	}

	snapshotID := r.Form["snapshotid"][0] //special id: "latest"
	restoreTarget := r.Form["target"][0]
	repoID, err := strconv.Atoi(r.Form["repoid"][0])
	defer http.Redirect(w, r, "/snapshot?id="+strconv.Itoa(repoID), http.StatusSeeOther)
	if len(snapshotID) == 0 || len(restoreTarget) == 0 || err != nil || restoreTarget == "null" {
		return
	}
	repo, err := getRepo(repoID)
	if err != nil {
		return
	}
	setActiveRepoEnvironmentVariables(repo.ID)
	restoreCmd := exec.Command(resticPath, "-r", repo.Location, "restore", snapshotID, "--target", restoreTarget)
	err = restoreCmd.Run()
	if err != nil {
		msg.setError(err.Error())
	} else {
		if configData.Settings.Language == "german" {
			msg.setSuccess("Snapshot wiederhergestellt")
		} else {
			msg.setSuccess("Restored Snapshot")
		}
	}
}

//restoreFileHandler restores a single file
func restoreFileHandler(w http.ResponseWriter, r *http.Request) {
	if !passwordCorrect {
		http.Redirect(w, r, "/pw", http.StatusSeeOther)
		return
	}
	r.ParseForm()
	if configData.Settings.Language == "german" {
		msg.setError("Konnte Datei nicht wiederherstellen")
	} else {
		msg.setError("Couldn't restore File")
	}

	repoID, err := strconv.Atoi(r.Form["repoid"][0])
	snapshotID := r.Form["snapshotid"][0]
	defer http.Redirect(w, r, "/snapshotfile?repoid="+strconv.Itoa(repoID)+"&snapshotid="+snapshotID, http.StatusSeeOther)
	restoreTarget := r.Form["target"][0]
	file := r.Form["file"][0]
	if len(snapshotID) == 0 || len(restoreTarget) == 0 || len(file) == 0 || err != nil {
		return
	}
	repo, err := getRepo(repoID)
	if err != nil {
		return
	}
	setActiveRepoEnvironmentVariables(repo.ID)
	restoreCmd := exec.Command(resticPath, "-r", repo.Location, "restore", snapshotID, "--target", restoreTarget, "--include", file)
	err = restoreCmd.Run()
	if err != nil {
		msg.setError(err.Error())
	} else {
		if configData.Settings.Language == "german" {
			msg.setSuccess("Datei widerhergestellt")
		} else {
			msg.setSuccess("Restored File")
		}

	}
}

//manualBackupHandler creates a Snapshot without a BackupJob
func manualBackupHandler(w http.ResponseWriter, r *http.Request) {
	if !passwordCorrect {
		http.Redirect(w, r, "/pw", http.StatusSeeOther)
		return
	}
	if configData.Settings.Language == "german" {
		msg.setError("Konnte Datei nicht sichern")
	} else {
		msg.setError("Couldn't backup File")
	}

	defer http.Redirect(w, r, "/repository", http.StatusSeeOther)
	r.ParseForm()
	file := r.Form["file"][0]
	repoID, err := strconv.Atoi(r.Form["repoid"][0])
	if err != nil {
		return
	}
	repo, err := getRepo(repoID)
	if err != nil {
		return
	}
	setActiveRepoEnvironmentVariables(repo.ID)
	backupCmd := exec.Command(resticPath, "-r", repo.Location, "backup", file)
	backupCmd.Start()
	if configData.Settings.Language == "german" {
		msg.setSuccess("Datei gesichert")
	} else {
		msg.setSuccess("File backed up")
	}

}

//newBackupJobHandler displays a form for creating BackupJobs
func newBackupJobHandler(w http.ResponseWriter, r *http.Request) {
	if !passwordCorrect {
		http.Redirect(w, r, "/pw", http.StatusSeeOther)
		return
	}
	r.ParseForm()
	if r.Form["create"] == nil {
		data := &GuiData{Config: configData, Message: msg.get()}
		newBackupJobTemplate.Execute(w, data)
	} else {
		if configData.Settings.Language == "german" {
			msg.setError("Backup Aufgabe konnte nicht erstellt werden")
		} else {
			msg.setError("Couldn't create Backup Job")
		}
		if r.Form["repoid"] == nil {
			if configData.Settings.Language == "german" {
				msg.setError("Kein Archiv vorhanden")
			} else {
				msg.setError("Create a Repository first")
			}
			http.Redirect(w, r, "/repository", http.StatusSeeOther)
			return
		}
		id := rand.Int()
		for getBackupJobName(id) != "" {
			id = rand.Int()
		}
		files := r.Form["files"]
		files = modifyFileList(files)
		if r.Form["create"][0] == "Create" {
			defer http.Redirect(w, r, "/backupjob", http.StatusSeeOther)
		} else {
			defer http.Redirect(w, r, "/filebrowser?jobid="+strconv.Itoa(id), http.StatusSeeOther)
		}
		name := r.Form["name"][0]
		for i := 0; i < len(files); i++ {
			files[i] = strings.Replace(files[i], "\\\\", "\\", -1)
		}
		repoID, err := strconv.Atoi(r.Form["repoid"][0])
		if err != nil {
			return
		}
		if err != nil {
			msg.setError(err.Error())
			return
		}
		start, err := time.Parse("2006-01-02T15:04", r.Form["start"][0])
		if err != nil {
			msg.setError(err.Error())
		}
		if len(name) == 0 {
			return
		}
		scheduled := r.Form["schedule"] != nil
		var mailerror bool
		var mailsuccess bool
		if r.Form["mail"] != nil {
			switch r.Form["mail"][0] {
			case "always":
				mailerror, mailsuccess = true, true
			case "error":
				mailerror, mailsuccess = true, false
			default:
				mailerror, mailsuccess = false, false
			}
		} else {
			mailerror, mailsuccess = false, false
		}
		interval, _ := strconv.Atoi(r.Form["interval"][0])
		weeks := WeekSchedule{
			Interval: interval,
			MON:      r.Form["mon"] != nil,
			TUE:      r.Form["tue"] != nil,
			WED:      r.Form["wed"] != nil,
			THU:      r.Form["thu"] != nil,
			FRI:      r.Form["fri"] != nil,
			SAT:      r.Form["sat"] != nil,
			SUN:      r.Form["sun"] != nil,
		}
		readFromConfig()
		configData.BackupJobs = append(configData.BackupJobs, BackupJob{Name: name, Files: files, ID: id, Weeks: weeks, RepoID: repoID, Start: start, Scheduled: scheduled, MailError: mailerror, MailSuccess: mailsuccess})
		writeToConfig()
		err = createScheduledTask(configData.BackupJobs[len(configData.BackupJobs)-1])
		if err != nil {
			if configData.Settings.Language == "german" {
				msg.setError("Konnte Backup Aufgabe nicht erstellen. Sind Sie Administrator und auf Windows 10, 8 oder 7? (versuchen Sie 'Als Administrator ausführen')")
			} else {
				msg.setError("Couldn't create Backup Job. Are you administrator and on Windows 10, 8 or 7? (try 'Run as administrator')")
			}
			deleteBackupJob(configData.BackupJobs[len(configData.BackupJobs)-1].ID)
		} else {
			if configData.Settings.Language == "german" {
				msg.setSuccess("Backup Aufgabe erstellt")
			} else {
				msg.setSuccess("Created Backup Job")
			}
		}
	}
}

//editBackupJobHandler handles form and input for editing BackupJobs
func editBackupJobHandler(w http.ResponseWriter, r *http.Request) {
	if !passwordCorrect {
		http.Redirect(w, r, "/pw", http.StatusSeeOther)
		return
	}
	r.ParseForm()
	jobID, err := strconv.Atoi(r.Form["jobid"][0])
	if err != nil {
		return
	}
	if r.Form["edit"] == nil {
		readFromConfig()
		job, err := getBackupJob(jobID)
		if err != nil {
			return
		}
		data := &GuiData{CurrentBackupJob: *job, Config: configData, Message: msg.get()}
		editBackupJobTemplate.Execute(w, data)
	} else {
		if configData.Settings.Language == "german" {
			msg.setError("Konnte Backup Aufgabe nicht bearbeiten")
		} else {
			msg.setError("Couldn't edit Backup Job")
		}

		readFromConfig()
		defer writeToConfig()
		files := r.Form["files"]
		for i := 0; i < len(files); i++ {
			files[i] = strings.Replace(files[i], "\\\\", "\\", -1)
		}
		jobIndexToEdit := -1
		for index, job := range configData.BackupJobs {
			if jobID == job.ID {
				jobIndexToEdit = index
				break
			}
		}
		if jobIndexToEdit == -1 {
			return
		}
		files = modifyFileList(files)
		if r.Form["edit"][0] != "Save" {
			defer http.Redirect(w, r, "/filebrowser?jobid="+strconv.Itoa(jobID), http.StatusSeeOther)
		} else {
			defer http.Redirect(w, r, "/backupjob", http.StatusSeeOther)
		}
		if r.Form["edit"][0] == "add" {
			configData.BackupJobs[jobIndexToEdit].Files = append(configData.BackupJobs[jobIndexToEdit].Files, files[0])
			if configData.Settings.Language == "german" {
				msg.setSuccess("Datei hinzugefügt")
			} else {
				msg.setSuccess("Added File")
			}

			return
		}
		if r.Form["edit"][0] == "remove" {
			for i := 0; i < len(configData.BackupJobs[jobIndexToEdit].Files); i++ {
				if configData.BackupJobs[jobIndexToEdit].Files[i] == files[0] {
					configData.BackupJobs[jobIndexToEdit].Files = append(configData.BackupJobs[jobIndexToEdit].Files[:i], configData.BackupJobs[jobIndexToEdit].Files[i+1:]...)
					if configData.Settings.Language == "german" {
						msg.setSuccess("Datei entfernt")
					} else {
						msg.setSuccess("Removed File")
					}
					if len(configData.BackupJobs[jobIndexToEdit].Files) < 1 {
						configData.BackupJobs[jobIndexToEdit].Files = append(configData.BackupJobs[jobIndexToEdit].Files, "")
					}
					return
				}
			}
			if configData.Settings.Language == "german" {
				msg.setError("Konnte Datei nicht entfernen")
			} else {
				msg.setError("Couldn't remove file")
			}

			return
		}
		repoID, err := strconv.Atoi(r.Form["repoid"][0])
		name := r.Form["name"][0]
		if len(name) == 0 || len(files) == 0 || err != nil {
			return
		}
		interval, _ := strconv.Atoi(r.Form["interval"][0])
		weeks := WeekSchedule{
			Interval: interval,
			MON:      r.Form["mon"] != nil,
			TUE:      r.Form["tue"] != nil,
			WED:      r.Form["wed"] != nil,
			THU:      r.Form["thu"] != nil,
			FRI:      r.Form["fri"] != nil,
			SAT:      r.Form["sat"] != nil,
			SUN:      r.Form["sun"] != nil,
		}
		scheduled := r.Form["schedule"] != nil
		var mailerror bool
		var mailsuccess bool
		if r.Form["mail"] != nil {
			switch r.Form["mail"][0] {
			case "always":
				mailerror, mailsuccess = true, true
			case "error":
				mailerror, mailsuccess = true, false
			default:
				mailerror, mailsuccess = false, false
			}
		} else {
			mailerror, mailsuccess = false, false
		}
		start, err := time.Parse("2006-01-02T15:04", r.Form["start"][0])
		if err != nil {
			msg.setError(err.Error())
		}

		configData.BackupJobs[jobIndexToEdit].Name = name
		configData.BackupJobs[jobIndexToEdit].Files = files
		configData.BackupJobs[jobIndexToEdit].RepoID = repoID
		configData.BackupJobs[jobIndexToEdit].Weeks = weeks
		configData.BackupJobs[jobIndexToEdit].Start = start
		configData.BackupJobs[jobIndexToEdit].Scheduled = scheduled
		configData.BackupJobs[jobIndexToEdit].MailError = mailerror
		configData.BackupJobs[jobIndexToEdit].MailSuccess = mailsuccess
		writeToConfig()
		err = editScheduledTask(configData.BackupJobs[jobIndexToEdit])
		if err != nil {
			msg.setError(err.Error())
		} else {
			if configData.Settings.Language == "german" {
				msg.setSuccess("Backup Aufgabe bearbeitet")
			} else {
				msg.setSuccess("Edited Backup Job")
			}

		}
	}
}

//forgetHandler deletes everything but the last n snapshots
func forgetHandler(w http.ResponseWriter, r *http.Request) {
	if !passwordCorrect {
		http.Redirect(w, r, "/pw", http.StatusSeeOther)
		return
	}
	r.ParseForm()
	repoID, err := strconv.Atoi(r.Form["repoid"][0])
	values := r.Form["value"]       //value can be numbers or tags
	modifiers := r.Form["modifier"] //modifiers can be "last", "hourly", "daily", "weekly", "monthly", "yearly", "tag"
	defer http.Redirect(w, r, "/snapshot?id="+strconv.Itoa(repoID), http.StatusSeeOther)
	if err != nil || len(values) == 0 || len(modifiers) == 0 {
		return
	}
	repo, err := getRepo(repoID)
	if err != nil {
		return
	}
	setActiveRepoEnvironmentVariables(repo.ID)
	forgetCmd := exec.Command(resticPath, "-r", repo.Location, "forget")
	for idx, modifier := range modifiers {
		forgetCmd.Args = append(forgetCmd.Args, "--keep-"+modifier, values[idx])
	}
	forgetCmd.Args = append(forgetCmd.Args, "--prune")
	forgetCmd.Run()
}

//settingsHandler handles the settings and actions
func settingsHandler(w http.ResponseWriter, r *http.Request) {
	if !passwordCorrect {
		http.Redirect(w, r, "/pw", http.StatusSeeOther)
		return
	}
	r.ParseForm()
	if r.Form["save"] != nil { // User pressed Save-Button (otherwise User wants to view settings)
		readFromConfig()
		if configData.Settings.Language == "german" {
			msg.setError("Einstellungen konnten nicht gespeichert werden")
		} else {
			msg.setError("Settings not saved")
		}

		if r.Form["language"] != nil {
			configData.Settings.Language = r.Form["language"][0]
		} else {
			configData.Settings.Language = "english"
		}
		parseAllTemplates()
		configData.Settings.OpenBrowser = r.Form["openbrowser"] != nil
		configData.Settings.AutomaticPageReload = r.Form["autoreload"] != nil
		configData.Settings.AutomaticScroll = r.Form["autoscroll"] != nil
		configData.Settings.ShowMessages = r.Form["messages"] != nil
		configData.Settings.Tips = r.Form["tips"] != nil
		if r.Form["password"] != nil {
			configData.Settings.Password = ""
			passwordCorrect = false
		}
		configData.Settings.Mail.SendAfterBackupComplete = r.Form["sendmail"] != nil
		configData.Settings.Mail.Hostname = r.Form["hostname"][0]
		configData.Settings.Mail.Port = r.Form["port"][0]
		configData.Settings.Mail.Username = r.Form["username"][0]
		configData.Settings.Mail.Password = r.Form["mailpassword"][0]
		configData.Settings.Mail.From = r.Form["from"][0]
		configData.Settings.Mail.Recipient = r.Form["recipient"][0]
		writeToConfig()
		if configData.Settings.Language == "german" {
			msg.setSuccess("Einstellungen gespeichert")
		} else {
			msg.setSuccess("Settings saved")
		}
		if r.Form["save"][0] == "Send" {
			if configData.Settings.Language == "german" {
				sendMail("Hallo!\r\nWenn Sie diese Nachricht sehen, können sie ab sofort Emails von FuseNX erhalten. Wenn Sie zu Backup Aufgaben Emails erhalten wollen, erlauben Sie das Senden von Emails in den Einstellungen und bei den einzelnen Backupaufgaben.", "FuseNX Test Email")
			} else {
				sendMail("Hello!\r\nIf you see this, you are ready to receive mails from FuseNX. If you wan to receive emails for Backup Jobs, allow sending Emails in settings and for chosen Backup Jobs.", "FuseNX Test Email")
			}
		}
		if !passwordCorrect {
			http.Redirect(w, r, "/pw", http.StatusSeeOther)
			return
		}
	}
	data := &GuiData{Config: configData, Message: msg.get(), Version: version}
	settingsTemplate.Execute(w, data)
}

//passwordHandler requests password input
func passwordHandler(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	readFromConfig()
	if configData.Settings.Language == "" {
		for _, lang := range strings.Split(r.Header.Get("Accept-Language"), ";") {
			if strings.Contains(strings.ToLower(lang), "de") {
				configData.Settings.Language = "german"
				break
			} else if strings.Contains(lang, "en") {
				configData.Settings.Language = "english"
				break
			}
		}
		if configData.Settings.Language == "" {
			configData.Settings.Language = "english"
		}
		parseAllTemplates()
	}
	if r.Form["logout"] != nil {
		passwordCorrect = false
		http.Redirect(w, r, "/pw", http.StatusSeeOther)
	}
	if r.Form["forgot"] != nil {
		if configData.Settings.Language == "german" {
			sendMail("Ihr FuseNX Passwort ist: "+configData.Settings.Password+" \r\nBitte ändern Sie es sofort.", "FuseNX Passwort")
		} else {
			sendMail("Your FuseNX password is: "+configData.Settings.Password+" \r\nPlease change it immediately.", "FuseNX Password")
		}
		http.Redirect(w, r, "/pw", http.StatusSeeOther)
		return
	}
	if passwordCorrect {
		http.Redirect(w, r, "/gui", http.StatusSeeOther)
	}
	if r.Form["password"] == nil {
		data := &GuiData{Config: configData, Message: msg.get(), IsAdmin: isAdmin}
		passwordTemplate.Execute(w, data)
	} else {
		if configData.Settings.Password != "" || r.Form["password"][0] == r.Form["password2"][0] {
			validatePassword(r.Form["password"][0])
		}
		if passwordCorrect {
			if configData.Settings.Language == "german" {
				msg.set("Willkommen")
			} else {
				msg.set("Welcome")
			}

			http.Redirect(w, r, "/gui", http.StatusSeeOther)
		} else {
			if configData.Settings.Language == "german" {
				msg.setError("Falsches Passwort")
			} else {
				msg.setError("Wrong password")
			}
			data := &GuiData{Config: configData, Message: msg.get(), IsAdmin: isAdmin}
			passwordTemplate.Execute(w, data)
		}
	}
}

//logHandler handles displaying log data
func logHandler(w http.ResponseWriter, r *http.Request) {
	if !passwordCorrect {
		http.Redirect(w, r, "/pw", http.StatusSeeOther)
		return
	}
	readFromConfig()
	r.ParseForm()
	jobID, err := strconv.Atoi(r.Form["jobid"][0])
	if err != nil {
		return
	}
	backupJob, err := getBackupJob(jobID)
	if err != nil {
		return
	}
	data := &GuiData{Config: configData, Message: msg.get(), CurrentBackupJob: *backupJob}
	logTemplate.Execute(w, data)
}

//keepAliveHandler resets the shutdown time
func keepAliveHandler(w http.ResponseWriter, r *http.Request) {
	shutdownTime = time.Now().Add(time.Second * 5)
}

func versionCheckHandler(w http.ResponseWriter, r *http.Request) {
	response, err := http.Get(versionCheckURL + "?v=" + fmt.Sprint(version))
	if err != nil {
		msg.setError(err.Error())
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		return
	}
	defer response.Body.Close()
	body, _ := ioutil.ReadAll(response.Body)
	if string(body) == "true" {
		http.Redirect(w, r, "/settings", http.StatusSeeOther)
		if configData.Settings.Language == "german" {
			msg.set("Neueste Version")
		} else {
			msg.set("Newest version")
		}
	} else {
		http.Redirect(w, r, newerVersionDownload, http.StatusSeeOther)
		if configData.Settings.Language == "german" {
			msg.set("Neuere Version verfügbar")
		} else {
			msg.set("Newer version available")
		}
	}
}

//fileBrowserHandler handles viewing the file tree
func fileBrowserHandler(w http.ResponseWriter, r *http.Request) {
	if !passwordCorrect {
		http.Redirect(w, r, "/pw", http.StatusSeeOther)
		return
	}
	r.ParseForm()
	var directory string
	var repo Repo
	var job BackupJob
	if r.Form["curDir"] != nil {
		directory = strings.Replace(r.Form["curDir"][0], "\\\\", "\\", -1)
	} else {
		switch runtime.GOOS {
		case "linux":
			directory = filepath.Dir("/")
		case "windows":
			directory = filepath.Dir("C:\\")
		}
	}
	if r.Form["new"] != nil && r.Form["new"][0] != "null" {
		os.MkdirAll(directory+"\\"+r.Form["new"][0], 0777)
	}
	if r.Form["repoid"] != nil {
		repoID, err := strconv.Atoi(r.Form["repoid"][0])
		if err == nil {
			repoPtr, err := getRepo(repoID)
			if err == nil {
				repo = *repoPtr
			}
		}
	}
	if r.Form["jobid"] != nil {
		jobID, err := strconv.Atoi(r.Form["jobid"][0])
		if err == nil {
			jobPtr, err := getBackupJob(jobID)
			if err == nil {
				job = *jobPtr
			}
		}
	}
	files := readDirectory(directory)
	data := &GuiData{Files: files, CurrentDirectory: directory, CurrentRepo: repo, CurrentBackupJob: job, Message: msg.get(), Config: configData}
	filebrowserTemplate.Execute(w, data)
}

func getDirectoryHandler(w http.ResponseWriter, r *http.Request) {
	if !passwordCorrect {
		http.Redirect(w, r, "/pw", http.StatusSeeOther)
		return
	}
	r.ParseForm()
	var directory string
	var repo Repo
	var job BackupJob
	if r.Form["curDir"] == nil {
		return
	}
	directory = filepath.Clean(r.Form["curDir"][0])
	if r.Form["repoid"] != nil {
		repoID, err := strconv.Atoi(r.Form["repoid"][0])
		if err == nil {
			repoPtr, err := getRepo(repoID)
			if err == nil {
				repo = *repoPtr
			}
		}
	}
	if r.Form["jobid"] != nil {
		jobID, err := strconv.Atoi(r.Form["jobid"][0])
		if err == nil {
			jobPtr, err := getBackupJob(jobID)
			if err == nil {
				job = *jobPtr
			}
		}
	}
	files := readDirectory(directory)
	data := &GuiData{Files: files, CurrentDirectory: directory, CurrentRepo: repo, CurrentBackupJob: job}
	directoryListTemplate.Execute(w, data)
}

func checkHandler(w http.ResponseWriter, r *http.Request) {
	if !passwordCorrect {
		http.Redirect(w, r, "/pw", http.StatusSeeOther)
		return
	}
	msg.setError("Check failed")
	r.ParseForm()
	repoID, err := strconv.Atoi(r.Form["repoid"][0])
	defer http.Redirect(w, r, "/repository", http.StatusSeeOther)
	if err != nil {
		return
	}
	repo, err := getRepo(repoID)
	if err != nil {
		return
	}
	setActiveRepoEnvironmentVariables(repo.ID)
	checkCmd := exec.Command(resticPath, "-r", repo.Location, "check")
	output, err := checkCmd.CombinedOutput()
	if err != nil {
		msg.setError(string(output))
		return
	}
	msg.setSuccess(string(output))
	fmt.Println(string(output))
}
