package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	assetfs "github.com/elazarl/go-bindata-assetfs"
)

//HTML Templates
var (
	indexTemplate          *template.Template
	filebrowserTemplate    *template.Template
	snapshotTemplate       *template.Template
	settingsTemplate       *template.Template
	helpTemplate           *template.Template
	repositoryTemplate     *template.Template
	newRepositoryTemplate  *template.Template
	editRepositoryTemplate *template.Template
	backupJobTemplate      *template.Template
	newBackupJobTemplate   *template.Template
	editBackupJobTemplate  *template.Template
	snapshotFileTemplate   *template.Template
	passwordTemplate       *template.Template
	logTemplate            *template.Template
)

var (
	configData      ConfigData //data from config file
	msg             message    //for error messages or information to be displayed in the web interface
	passwordCorrect bool
	configFilePath  string
	shutdownTime    time.Time
	isAdmin         bool
)

const (
	defaultPassword      string  = "G6DFxBFm1YUEHgqBxdxjymMZuZykxgT5"
	version              float64 = 1.0
	versionCheckURL      string  = "http://localhost:81/isnewestversion.php"
	newerVersionDownload string  = "http://localhost:81"
)

func main() {
	initConfig()
	os.Setenv("Path", os.Getenv("Path")+";"+filepath.Dir(os.Args[0])) // sets Path to include restic next to this executable
	if err := exec.Command("restic", "help").Start(); err != nil {
		//restic not found
		ioutil.WriteFile("restic.exe", MustAsset("data/bin/restic.exe"), 0777)
	}
	if len(os.Args) > 2 { // invoked by Windows Task Scheduler
		if os.Args[1] == "job" || os.Args[1] == "-job" || os.Args[1] == "/job" {
			os.Chdir(filepath.Dir(os.Args[0]))
			performScheduledBackup()
		}
		os.Exit(0)
	}
	serveGUI()
}

//serveGUI parses all templates and starts the server
func serveGUI() {
	switch configData.Settings.Language {
	case "english":
		fmt.Println("Do not close this window until you are done with FuseNX.")
	case "german":
		fmt.Println("Schließen Sie dieses Fenster nicht wenn Sie FuseNX noch verwenden.")
	default:
		fmt.Println("Do not close this window until you are done with FuseNX.")
	}
	if configData.Settings.OpenBrowser {
		go openDefaultBrowser()
	} else {
		switch configData.Settings.Language {
		case "english":
			fmt.Println("Visit http://localhost/gui to see the GUI (Go to settings to enable automatic browser start)")
		case "german":
			fmt.Println("Besuchen Sie http://localhost/gui um das GUI zu sehen (automatischer Start des Browsers in den Einstellungen aktivierbar)")
		}
	}
	go func() { // Testing for admin permission
		_, err := exec.Command("schtasks", "/create", "/tn", "Eitea FuseNX Test Task", "/tr", "cmd", "/sc", "weekly", "/sd", time.Now().Format("02/01/2006"), "/st", time.Now().Add(-5*time.Minute).Format("15:04"), "/ru", "SYSTEM", "/f").CombinedOutput()
		exec.Command("schtasks", "/delete", "/tn", "Eitea FuseNX Test Task", "/f").CombinedOutput()
		isAdmin = err == nil
	}()

	parseAllTemplates()

	http.HandleFunc("/", indexHandler)            //start page
	http.HandleFunc("/help", helpHandler)         //help page
	http.HandleFunc("/settings", settingsHandler) //settings page
	http.HandleFunc("/exit", exitHandler)         //exits the program
	http.HandleFunc("/pw", passwordHandler)       //logging in and setting password
	http.HandleFunc("/alive", keepAliveHandler)   //used to detect closed tabs

	http.HandleFunc("/versioncheck", versionCheckHandler) //Checks version

	http.HandleFunc("/backup", executeBackupHandler)      //handler for manually executing a backup
	http.HandleFunc("/restore", restoreSnapshotHandler)   //handler for restoring whole snapshots
	http.HandleFunc("/restorefile", restoreFileHandler)   //handler for restoring single files/folders
	http.HandleFunc("/modifytag", modifyTagHandler)       //handler for adding/removing tags
	http.HandleFunc("/filebrowser", fileBrowserHandler)   //lets users browse files
	http.HandleFunc("/manualbackup", manualBackupHandler) //back up a file from filebrowser
	http.HandleFunc("/forget", forgetHandler)             //restics forget command

	http.HandleFunc("/snapshot", snapshotListHandler)         //list Snapshots
	http.HandleFunc("/deletesnapshot", deleteSnapshotHandler) //handler for deleting snapshots
	http.HandleFunc("/snapshotfile", listSnapshotFileHandler) //list files in a snapshot

	http.HandleFunc("/backupjob", backupJobHandler)             //list BackupJobs
	http.HandleFunc("/newbackupjob", newBackupJobHandler)       //form for creating BackupJobs
	http.HandleFunc("/deletebackupjob", deleteBackupJobHandler) //handler for deleting BackupJobs
	http.HandleFunc("/editbackupjob", editBackupJobHandler)     //form and input handler for editing BackupJobs
	http.HandleFunc("/log", logHandler)                         //display logs

	http.HandleFunc("/repository", repositoryHandler)             //list Repos
	http.HandleFunc("/newrepository", newRepositoryHandler)       //form for creating Repos
	http.HandleFunc("/deleterepository", deleteRepositoryHandler) //handler for deleting Repos and their BackupJobs
	http.HandleFunc("/editrepository", editRepositoryHandler)     //form and inpu handler for editing Repos

	go exitTimer()

	//Static Files (like Bootstrap/JQuery/style/favicon)
	http.Handle("/staticfiles/", http.StripPrefix("/staticfiles/", http.FileServer(&assetfs.AssetFS{Asset: Asset, AssetDir: AssetDir, AssetInfo: AssetInfo, Prefix: "data/staticfiles"})))

	//Start Server
	http.ListenAndServe(":80", nil)
	for i := 81; i < 60000; i++ {
		if configData.Settings.Language == "german" {
			fmt.Print("\nPort ", i-1, " ist nicht verfügbar. Versuche Port ", i, " (besuche http://localhost:", i, "/gui um die Benutzeroberfläche zu sehen).\n")
		} else {
			fmt.Print("\nPort ", i-1, " not available. Trying Port ", i, " (visit http://localhost:", i, "/gui to view the GUI).\n")
		}
		http.ListenAndServe(":"+strconv.Itoa(i), nil)
	}
}
