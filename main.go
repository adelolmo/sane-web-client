package main

import (
	"encoding/json"
	"fmt"
	"github.com/gobuffalo/packr"
	"github.com/gorilla/mux"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path"
	"sort"
	"strconv"
	"strings"
)

type settings struct {
	Mode       string `json:"mode"`
	Format     string `json:"format"`
	Resolution string `json:"resolution"`
	Updated    bool   `json:"-"`
}

type index struct {
	Title        string
	JobName      string
	PreviousJobs []string
}

type scanner struct {
	JobName string
	Scans   []string
}

var indexTemplate *template.Template
var jobTemplate *template.Template
var jobsTemplate *template.Template
var settingsTemplate *template.Template
var outputDirectory string

func init() {
	box := packr.NewBox("./templates")
	indexFile, err := box.FindString("index.html")
	if err != nil {
		log.Fatalln(err)
	}
	headerFile, err := box.FindString("header.html")
	if err != nil {
		log.Fatalln(err)
	}
	indexTemplate = template.Must(template.Must(template.New("index").Parse(headerFile)).Parse(indexFile))

	jobFile, err := box.FindString("job.html")
	if err != nil {
		log.Fatalln(err)
	}
	jobTemplate = template.Must(template.Must(template.New("job").Parse(headerFile)).Parse(jobFile))

	jobsFile, err := box.FindString("jobs.html")
	if err != nil {
		log.Fatalln(err)
	}
	jobsTemplate = template.Must(template.Must(template.New("jobs").Parse(headerFile)).Parse(jobsFile))

	settingsFile, err := box.FindString("settings.html")
	if err != nil {
		log.Fatalln(err)
	}
	settingsTemplate = template.Must(template.Must(template.New("settings").Parse(headerFile)).Parse(settingsFile))
}

func main() {
	port := os.Getenv("port")
	if port == "" {
		port = "8000"
	}
	outputDirectory = os.Getenv("outputDir")
	fmt.Println(fmt.Sprintf("port: %s, outputDir: %s", port, outputDirectory))

	box := packr.NewBox("assets/")

	router := mux.NewRouter()
	router.PathPrefix("/assets/").Handler(http.StripPrefix("/assets/", http.FileServer(box)))
	router.HandleFunc("/", homePage).Methods("GET")
	router.HandleFunc("/settings", showSettingsPage).Methods("GET")
	router.HandleFunc("/settings", updateSettingsPage).Methods("POST")
	router.HandleFunc("/jobs", showJobPage).Methods("GET")
	router.HandleFunc("/job", resumeJobPage).Methods("GET")
	router.HandleFunc("/job", createJobHandler).Methods("POST")
	router.HandleFunc("/scan", scanHandler).Methods("POST")
	router.HandleFunc("/scan", getFileHandler).Methods("GET")
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%s", port), router))
}

func homePage(w http.ResponseWriter, r *http.Request) {
	var previousJobs []string
	for _, dir := range jobDirectories() {
		previousJobs = append(previousJobs, dir.Name())
	}

	index := &index{
		PreviousJobs: previousJobs,
	}
	err := indexTemplate.Execute(w, index)
	if err != nil {
		panic(err)
	}
}

func showSettingsPage(w http.ResponseWriter, r *http.Request) {
	if _, err := os.Stat("/home/adelolmo/tmp/settings.json"); os.IsNotExist(err) {
		if err = settingsTemplate.Execute(w, &settings{}); err != nil {
			log.Fatalln(err)
		}
		return
	}
	file, err := ioutil.ReadFile("/home/adelolmo/tmp/settings.json")
	if err != nil {
		log.Fatalln(err)
	}
	settings := &settings{}
	if err = json.Unmarshal(file, settings); err != nil {
		log.Fatalln(err)
	}

	err = settingsTemplate.Execute(w, settings)
	if err != nil {
		panic(err)
	}
}

func updateSettingsPage(w http.ResponseWriter, r *http.Request) {
	mode := r.FormValue("mode")
	format := r.FormValue("format")
	resolution := r.FormValue("resolution")
	settings := &settings{
		Mode:       mode,
		Format:     format,
		Resolution: resolution,
		Updated:    true,
	}
	settingsJson, _ := json.Marshal(settings)
	if err := ioutil.WriteFile("/home/adelolmo/tmp/settings.json", settingsJson, 0644); err != nil {
		log.Fatalln(err)
	}

	if err := settingsTemplate.Execute(w, settings); err != nil {
		log.Fatalln(err)
	}
}

func showJobPage(w http.ResponseWriter, r *http.Request) {
	var previousJobs []string
	for _, dir := range jobDirectories() {
		previousJobs = append(previousJobs, dir.Name())
	}

	index := &index{
		PreviousJobs: previousJobs,
	}
	err := jobsTemplate.Execute(w, index)
	if err != nil {
		panic(err)
	}
}

func resumeJobPage(w http.ResponseWriter, r *http.Request) {
	jobName := r.FormValue("jobName")

	if err := os.MkdirAll(path.Join(outputDirectory, jobName), os.ModePerm); err != nil {
		log.Fatalln(err)
	}

	var scans []string
	directory := filesOnDirectory(path.Join(outputDirectory, jobName))
	for _, file := range directory {
		scans = append(scans, file.Name())
		println(file.Name())
	}

	scanner := &scanner{
		Scans:   scans,
		JobName: jobName,
	}
	if err := jobTemplate.Execute(w, scanner); err != nil {
		log.Fatalln(err)
	}
}

func createJobHandler(w http.ResponseWriter, r *http.Request) {
	jobName := r.FormValue("jobName")
	if err := os.MkdirAll(path.Join(outputDirectory, jobName), os.ModePerm); err != nil {
		log.Fatalln(err)
	}

	var scans []string
	for _, file := range filesOnDirectory(path.Join(outputDirectory, jobName)) {
		scans = append(scans, file.Name())
	}

	scanner := &scanner{
		Scans:   scans,
		JobName: jobName,
	}
	if err := jobTemplate.Execute(w, scanner); err != nil {
		log.Fatalln(err)
	}
}

func scanHandler(w http.ResponseWriter, r *http.Request) {
	jobName := r.FormValue("jobName")
	previousScans := filesOnDirectory(path.Join(outputDirectory, jobName))

	scanName := "1.tiff"
	/*for _, scanFilename := range previousScans {
	}*/
	if len(previousScans) > 0 {
		lastScanName := previousScans[len(previousScans)-1].Name()
		lastScanNumber, err := strconv.Atoi(strings.Split(lastScanName, ".")[0])
		if err != nil {
			println(err)
		}
		scanName = fmt.Sprintf("%d.tiff", lastScanNumber+1)
	}

	err := scan(path.Join(outputDirectory, jobName, scanName))
	if err != nil {
		log.Fatalln(err)
	}

	var scans []string
	for _, file := range previousScans {
		scans = append(scans, file.Name())
	}

	scanner := &scanner{
		Scans:   scans,
		JobName: jobName,
	}
	if err := jobTemplate.Execute(w, scanner); err != nil {
		log.Fatalln(err)
	}
}

func getFileHandler(w http.ResponseWriter, r *http.Request) {
	jobName := r.FormValue("jobName")
	scan := r.FormValue("scan")

	file, err := ioutil.ReadFile(path.Join(outputDirectory, jobName, scan))
	if err != nil {
		log.Fatalln(err)
	}
	/*	reader := bytes.NewReader(file)
		image, err := tiff.Decode(reader)
		if err != nil {
			log.Fatalln(err)
		}


		options := &tiff.Options{
			Compression: tiff.Uncompressed,
		}
		buffer := new(bytes.Buffer)
		if err = tiff.Encode(buffer, image, options); err != nil {
			log.Println("unable to encode image.")
		}*/

	w.Header().Set("Content-Type", "image/tiff")
	w.Header().Set("Content-Length", strconv.Itoa(len(file)))
	if _, err := w.Write(file); err != nil {
		log.Println("unable to write image.")
	}

	/*		i, err := w.Write(file)
			if err != nil {
				log.Fatalln(err)
			}
			println(i)*/
}

func scan(path string) error {
	// su -s /bin/sh - saned
	out, err := exec.Command("/usr/bin/scanimage",
		"--mode=Color", "--resolution=300", "--format=tiff").Output()
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(path, out, 0644)
	if err != nil {
		return err
	}
	return nil
}

func jobDirectories() []os.FileInfo {
	files, err := ioutil.ReadDir(outputDirectory)
	if err != nil {
		log.Fatal(err)
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime().After(files[j].ModTime())
	})
	i := 0
	for _, file := range files {
		if file.IsDir() {
			files[i] = file
			i++
		}
	}
	files = files[:i]
	return files
}

func filesOnDirectory(dir string) []os.FileInfo {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}
	sort.Slice(files, func(i, j int) bool {
		//return sort.Strings(files[i].Name(), files[j].Name())
		return files[i].Name() > files[j].Name()
	})
	//sort.Strings()
	i := 0
	for _, file := range files {
		if !file.IsDir() {
			files[i] = file
			i++
		}
	}
	files = files[:i]
	return files
}
