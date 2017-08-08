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
	backupCmd := exec.Command("restic", "-r", repo.Location, "backup")
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
	cmd := exec.Command("powershell.exe", "-NoExit", "-Command", "-")
	stdin, _ := cmd.StdinPipe()
	stdErr, _ := cmd.StderrPipe()
	path := os.Args[0]
	cmd.Start()
	stdin.Write([]byte("$Trigger = New-ScheduledTaskTrigger -Once -At \"" + job.Start.Format("01/02/2006 15:04:05") + "\" -RepetitionInterval (New-TimeSpan -Hours " + strconv.Itoa(int(job.Repeat.Hours())) + " -Minutes " + strconv.Itoa(int(job.Repeat.Minutes())%60) + " -Seconds " + strconv.Itoa(int(job.Repeat.Seconds())%60) + ")" + "\r\n"))
	stdin.Write([]byte("$Action = New-ScheduledTaskAction -Execute \"" + path + "\" -Argument \"job  " + strconv.Itoa(job.ID) + "\"" + "\r\n"))
	stdin.Write([]byte("$InputObject = New-ScheduledTask -Action $Action -Trigger $Trigger" + "\r\n"))
	stdin.Write([]byte(`Register-ScheduledTask -TaskName "Eitea FuseNX job ` + strconv.Itoa(job.ID) + `" -InputObject $InputObject -User "NT AUTHORITY\SYSTEM"` + "\r\n"))
	stdin.Close()
	errorMessage := make([]byte, 1000)
	bytesRead, _ := stdErr.Read(errorMessage)
	errorMessage = errorMessage[:bytesRead]
	if bytesRead > 1 {
		_, err := exec.Command("schtasks", "/create", "/tn", "Eitea FuseNX job "+strconv.Itoa(job.ID), "/tr", path+" job "+strconv.Itoa(job.ID), "/sc", "ONCE", "/sd", job.Start.Format("02/01/2006"), "/st", job.Start.Format("15:04"), "/ri", strconv.Itoa(int(job.Repeat.Minutes())), "/du", "9999:59", "/ru", "SYSTEM").CombinedOutput()
		if err != nil {
			return errors.New(string(errorMessage))
		}
	}
	return nil
}

//editScheduledTask edits a task
func editScheduledTask(job BackupJob) error {
	cmd := exec.Command("powershell.exe", "-NoExit", "-Command", "-")
	stdin, _ := cmd.StdinPipe()
	stdErr, _ := cmd.StderrPipe()
	path := os.Args[0]
	cmd.Start()
	stdin.Write([]byte("$Trigger = New-ScheduledTaskTrigger -Once -At \"" + job.Start.Format("01/02/2006 15:04:05") + "\" -RepetitionInterval (New-TimeSpan -Hours " + strconv.Itoa(int(job.Repeat.Hours())) + " -Minutes " + strconv.Itoa(int(job.Repeat.Minutes())%60) + " -Seconds " + strconv.Itoa(int(job.Repeat.Seconds())%60) + ")" + "\r\n"))
	stdin.Write([]byte("$Action = New-ScheduledTaskAction -Execute \"" + path + "\" -Argument \"job  " + strconv.Itoa(job.ID) + "\"" + "\r\n"))
	stdin.Write([]byte(`Set-ScheduledTask "Eitea FuseNX job ` + strconv.Itoa(job.ID) + `" -Trigger $Trigger -Action $Action -User "NT AUTHORITY\SYSTEM"` + "\r\n"))
	stdin.Close()
	errorMessage := make([]byte, 1000)
	bytesRead, _ := stdErr.Read(errorMessage)
	errorMessage = errorMessage[:bytesRead]
	if bytesRead > 1 {
		_, err := exec.Command("schtasks", "/change", "/tn", "Eitea FuseNX job "+strconv.Itoa(job.ID), "/tr", path+" job "+strconv.Itoa(job.ID), "/sd", job.Start.Format("02/01/2006"), "/st", job.Start.Format("15:04"), "/ri", strconv.Itoa(int(job.Repeat.Minutes())), "/du", "9999:59", "/ru", "SYSTEM").CombinedOutput()
		if err != nil {
			return errors.New(string(errorMessage))
		}
	}
	return nil
}

//deleteScheduledTask deletes a task
func deleteScheduledTask(jobID int) {
	cmd := exec.Command("powershell.exe", "-NoExit", "-Command", "-")
	stdin, _ := cmd.StdinPipe()
	stderr, _ := cmd.StderrPipe()
	cmd.Start()
	stdin.Write([]byte(`Unregister-ScheduledTask -TaskName "Eitea FuseNX job ` + strconv.Itoa(jobID) + `" -Confirm:$false ` + "\r\n"))
	stdin.Close()
	if num, _ := stderr.Read(make([]byte, 100)); num > 1 {
		exec.Command("schtasks", "/delete", "/tn", "Eitea FuseNX job "+strconv.Itoa(jobID), "/f").CombinedOutput()
	}
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
