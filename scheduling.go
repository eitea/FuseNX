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
	"errors"
	"fmt"
	"io/ioutil"
	"net/smtp"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

//performScheduledBackup performs a Backup Job when invoked by Task Scheduler
func performScheduledBackup() {
	readFromConfig()
	jobID, err := strconv.Atoi(os.Args[2])
	if err != nil {
		os.Exit(1)
	}
	backupJob, err := getBackupJob(jobID)
	if err != nil {
		os.Exit(1)
	}
	if !backupJob.Scheduled {
		return
	}
	repo, err := getRepo(backupJob.RepoID)
	if err != nil {
		os.Exit(1)
	}
	setActiveRepoEnvironmentVariables(repo.ID)
	backupCmd := exec.Command(resticPath, "-r", repo.Location, "backup")
	backupCmd.Args = append(backupCmd.Args, backupJob.Files...)
	stdErr, _ := backupCmd.StderrPipe()
	output, runError := backupCmd.Output()
	stdErr.Close()
	errorMessage := make([]byte, 1000)
	bytesRead, _ := stdErr.Read(errorMessage)
	errorMessage = errorMessage[:bytesRead]
	if len(errorMessage) > 1 || runError != nil {
		backupCmd := exec.Command(filepath.Dir(os.Args[0])+"\\restic.exe", "-r", repo.Location, "backup")
		backupCmd.Args = append(backupCmd.Args, backupJob.Files...)
		stdErr, _ = backupCmd.StderrPipe()
		output, runError = backupCmd.Output()
		errorMessage = make([]byte, 1000)
		bytesRead, _ = stdErr.Read(errorMessage)
		errorMessage = errorMessage[:bytesRead]
	}
	if len(errorMessage) > 1 || runError != nil {
		backupJob.LatestRunSuccessful = false
		if configData.Settings.Mail.SendAfterBackupComplete && backupJob.MailError {
			if configData.Settings.Language == "german" {
				sendMail("Ausgabe: "+string(output)+"\r\nFehler: "+string(errorMessage)+"\r\n"+runError.Error(), "[FuseNX] Backup Aufgabe "+backupJob.Name+" "+os.Args[2])
			} else {
				sendMail("Output: "+string(output)+"\r\nError: "+string(errorMessage)+"\r\n"+runError.Error(), "[FuseNX] Backup Job "+backupJob.Name+" "+os.Args[2])
			}
		}
		appendLog(jobID, false, "Error: "+string(errorMessage))
		writeToConfig()
		ioutil.WriteFile("C:\\ProgramData\\eitea\\debug_error_log.txt", []byte(string(errorMessage)+"\n"+err.Error()), 0666)
		return
	}
	backupJob.LatestRunSuccessful = true
	if configData.Settings.Mail.SendAfterBackupComplete && backupJob.MailSuccess {
		if configData.Settings.Language == "german" {
			sendMail("Ausgabe: "+string(output), "[FuseNX] Backup Aufgabe "+backupJob.Name+" "+os.Args[2])
		} else {
			sendMail("Output: "+string(output), "[FuseNX] Backup Job "+backupJob.Name+" "+os.Args[2])
		}
	}
	appendLog(jobID, true, "Output: "+string(output))
	writeToConfig()
}

//createScheduledTask creates a scheduled task with Task Scheduler
func createScheduledTask(job BackupJob) error {
	path := os.Args[0]
	cmd := exec.Command("schtasks", "/create", "/tn", "Eitea FuseNX job "+strconv.Itoa(job.ID), "/tr", "\""+path+"\" job "+strconv.Itoa(job.ID), "/sc", "weekly", "/sd", job.Start.Format("02/01/2006"), "/st", job.Start.Format("15:04"), "/d", job.Weeks.formatDays(), "/mo", job.Weeks.formatInterval(), "/f")
	if isAdmin {
		cmd.Args = append(cmd.Args, "/ru", "SYSTEM")
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.New(err.Error() + " - " + string(output))
	}
	return nil
}

//editScheduledTask edits a task
func editScheduledTask(job BackupJob) error {
	return createScheduledTask(job)
}

//deleteScheduledTask deletes a task
func deleteScheduledTask(jobID int) {
	exec.Command("schtasks", "/delete", "/tn", "Eitea FuseNX job "+strconv.Itoa(jobID), "/f").CombinedOutput()
}

//sendMail sends a mail according to settings
func sendMail(body, subject string) {
	mailSettings := configData.Settings.Mail
	auth := smtp.PlainAuth("", mailSettings.Username, mailSettings.Password, mailSettings.Hostname)
	err := smtp.SendMail(
		mailSettings.Hostname+":"+mailSettings.Port,
		auth,
		mailSettings.From,
		[]string{mailSettings.Recipient},
		[]byte("From: "+mailSettings.From+"\r\nTo: "+mailSettings.Recipient+"\r\nSubject: "+subject+"\r\n\r\n"+body),
	)
	if err != nil {
		msg.setError(err.Error())
	} else {
		if configData.Settings.Language == "german" {
			msg.set("Nachricht wurde gesendet")
		} else {
			msg.set("Message sent")
		}
	}
}

//addJobCmd adds a Backup Job //id up to 1000 are reserved
func addJobCmd() { // fusenx add <id> <name> <repoid> <2006-01-02T15:04> <scheduled> <mailerror> <mailsuccess> <weekinterval> <mon> <tue> <wed> <thu> <fri> <sat> <sun> <files...>
	id, _ := strconv.Atoi(os.Args[2])
	name := os.Args[3]
	repoID, _ := strconv.Atoi(os.Args[4])
	start, _ := time.Parse("2006-01-02T15:04", os.Args[5])
	scheduled := os.Args[6] == "true"
	mailerror := os.Args[7] == "true"
	mailsuccess := os.Args[8] == "true"
	interval, _ := strconv.Atoi(os.Args[9])
	weeks := WeekSchedule{
		Interval: interval,
		MON:      os.Args[10] == "true",
		TUE:      os.Args[11] == "true",
		WED:      os.Args[12] == "true",
		THU:      os.Args[13] == "true",
		FRI:      os.Args[14] == "true",
		SAT:      os.Args[15] == "true",
		SUN:      os.Args[16] == "true",
	}
	files := os.Args[17:]
	for i := 0; i < len(files); i++ {
		files[i] = filepath.Clean(files[i])
	}
	readFromConfig()
	jobIndexToEdit := -1
	for index, job := range configData.BackupJobs {
		if id == job.ID {
			jobIndexToEdit = index
			break
		}
	}
	if jobIndexToEdit == -1 {
		jobIndexToEdit = len(configData.BackupJobs)
		configData.BackupJobs = append(configData.BackupJobs, BackupJob{ID: id})
	}
	configData.BackupJobs[jobIndexToEdit].Name = name
	configData.BackupJobs[jobIndexToEdit].Files = files
	configData.BackupJobs[jobIndexToEdit].Weeks = weeks
	configData.BackupJobs[jobIndexToEdit].RepoID = repoID
	configData.BackupJobs[jobIndexToEdit].Start = start
	configData.BackupJobs[jobIndexToEdit].Scheduled = scheduled
	configData.BackupJobs[jobIndexToEdit].MailError = mailerror
	configData.BackupJobs[jobIndexToEdit].MailSuccess = mailsuccess
	writeToConfig()
	err := createScheduledTask(configData.BackupJobs[jobIndexToEdit])
	if err != nil {
		os.Exit(1)
	}
	os.Exit(0)
}

//editRepocmd creates or edits a Repo //id up to 1000 are reserved for this
func editRepoCmd() { //fusenx repo <id> <name> <type> <location> <env> <password>
	readFromConfig()
	repoID, _ := strconv.Atoi(os.Args[2])
	repoIndexToEdit := -1
	for index, repo := range configData.Repos {
		if repoID == repo.ID {
			repoIndexToEdit = index
			break
		}
	}
	if repoIndexToEdit == -1 {
		repoIndexToEdit = len(configData.Repos)
		configData.Repos = append(configData.Repos, Repo{ID: repoID})
	}
	configData.Repos[repoIndexToEdit].Name = os.Args[3]
	configData.Repos[repoIndexToEdit].Type = os.Args[4]
	configData.Repos[repoIndexToEdit].Location = os.Args[5]
	configData.Repos[repoIndexToEdit].EnvVariables = strings.Split(os.Args[6], ",")
	configData.Repos[repoIndexToEdit].Password = os.Args[7]
	writeToConfig()
}

func deleteJobCmd() { //fusenx deletejob <id>
	jobID, _ := strconv.Atoi(os.Args[2])
	deleteBackupJob(jobID)
}

func deleteRepoCmd() { //fusenx deleterepo <id>
	repoID, _ := strconv.Atoi(os.Args[2])
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
		if job.running {
			fmt.Fprintln(os.Stderr, "A job is still running")
			os.Exit(1)
		}
		if job.RepoID == repoID {
			deleteBackupJob(job.ID)
		}
	}
	//cut the slice
	configData.Repos = append(configData.Repos[:repoIndexToDelete], configData.Repos[repoIndexToDelete+1:]...)
	writeToConfig()
}

func logCmd() { //fusenx log <id>
	jobID, _ := strconv.Atoi(os.Args[2])
	job, _ := getBackupJob(jobID)
	for _, log := range job.Logs {
		success := "Successful"
		if !log.Success {
			success = "Not Successful"
		}
		fmt.Println(formatTime(log.Time) + "\t" + success + "\t" + log.Text)
	}
}
