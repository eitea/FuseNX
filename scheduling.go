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
	"io/ioutil"
	"net/smtp"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
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
	cmd := exec.Command("schtasks", "/create", "/tn", "Eitea FuseNX job "+strconv.Itoa(job.ID), "/tr", path+" job "+strconv.Itoa(job.ID), "/sc", "weekly", "/sd", job.Start.Format("02/01/2006"), "/st", job.Start.Format("15:04"), "/d", job.Weeks.formatDays(), "/mo", job.Weeks.formatInterval(), "/f")
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
