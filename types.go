package main

import (
	"os"
	"strconv"
	"strings"
	"time"
)

//GuiData contains general infomation for all templates combined
type GuiData struct {
	Files             []FileInfoWithPath
	CurrentDirectory  string
	Snapshots         []Snapshot
	Config            ConfigData
	Message           string
	CurrentBackupJob  BackupJob //for editbackupjob
	CurrentRepo       Repo      //for editrepository
	CurrentSnapshotID string
	SnapshotFiles     []string
	Import            bool
	Version           float64
}

//FileInfoWithPath contains a FileInfo and a full path to the file
type FileInfoWithPath struct {
	FileInfo os.FileInfo
	Path     string
}

//Snapshot contains information about one particular restic snapshot
type Snapshot struct {
	ID       string
	Time     string
	Hostname string
	Tags     []string
	Paths    []string
	Username string
}

//Repo stores information about a repository
type Repo struct {
	Name         string
	ID           int
	Type         string //local, sftp, rest, s3, ninio, swift, b2
	Location     string
	EnvVariables []string //"variablename=variablevalue"
	Password     string
}

//BackupJob stores jobs
type BackupJob struct {
	Name                string
	Files               []string //files and folders to back up
	ID                  int
	RepoID              int
	Start               time.Time
	Weeks               WeekSchedule
	Logs                []Log
	LatestRunSuccessful bool
	Scheduled           bool
	MailError           bool //send email when error occurs
	MailSuccess         bool //send email when backup successful
}

//WeekSchedule represents the weekly trigger in schtasks
type WeekSchedule struct {
	Interval                          int
	SUN, MON, TUE, WED, THU, FRI, SAT bool
}

func (ws *WeekSchedule) formatDays() (out string) {
	if ws.MON {
		out += "MON,"
	}
	if ws.TUE {
		out += "TUE,"
	}
	if ws.WED {
		out += "WED,"
	}
	if ws.THU {
		out += "THU,"
	}
	if ws.FRI {
		out += "FRI,"
	}
	if ws.SAT {
		out += "SAT,"
	}
	if ws.SUN {
		out += "SUN,"
	}
	out = strings.TrimSuffix(out, ",")
	if len(out) == 0 {
		out = "SUN"
	}
	return
}

func (ws *WeekSchedule) formatInterval() (out string) {
	if ws.Interval <= 0 {
		out = strconv.Itoa(1)
	} else if ws.Interval >= 52 {
		out = strconv.Itoa(52)
	} else {
		out = strconv.Itoa(ws.Interval)
	}
	return
}

func (ws WeekSchedule) String() string {
	if configData.Settings.Language == "german" {
		return "Alle " + ws.formatInterval() + " Wochen am " + ws.formatDays()
	}
	return "Every " + ws.formatInterval() + " weeks on " + ws.formatDays()
}

//Log represents a logged BackupJob
type Log struct {
	Time    time.Time
	Text    string
	Success bool
}

//ConfigData saves repos and backupjobs
type ConfigData struct {
	Repos      []Repo
	BackupJobs []BackupJob
	Settings   Setting
}

//Setting contains all available settings for FuseNX
type Setting struct {
	OpenBrowser         bool
	AutomaticPageReload bool
	AutomaticScroll     bool
	Mail                Mail
	Password            string
	ShowMessages        bool
	Tips                bool
	Language            string //"english" or "german"
}

//Mail holds information about an smtp connection
type Mail struct {
	Hostname                string
	Port                    string
	Username                string
	Password                string
	From                    string
	Recipient               string
	SendAfterBackupComplete bool
}

//message is the text to display when the next page is viewed
type message struct {
	Value      string
	SeenByUser bool // whether shown to the user or not
}

//set sets message to "<msg>"
func (m *message) set(msg string) {
	m.Value = msg
	m.SeenByUser = false
}

//setError sets message to "Error: <msg>"
func (m *message) setError(msg string) {
	if configData.Settings.Language == "german" {
		m.Value = "Fehler: " + msg
	} else {
		m.Value = "Error: " + msg
	}

	m.SeenByUser = false
}

//setSuccess sets message to "Success: <msg>"
func (m *message) setSuccess(msg string) {
	if configData.Settings.Language == "german" {
		m.Value = "Erfolg: " + msg
	} else {
		m.Value = "Success: " + msg
	}

	m.SeenByUser = false
}

//get only returns a message when not already seen by user
func (m *message) get() string {
	if m.SeenByUser || !configData.Settings.ShowMessages {
		return ""
	}
	m.SeenByUser = true
	return m.Value
}
