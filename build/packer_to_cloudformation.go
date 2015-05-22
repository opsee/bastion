package main

import (
    "encoding/csv"
    "encoding/json"
    "flag"
    "os"
    "strings"
    "text/template"
)

var packerLog string
var cloudformation string

func init() {
    flag.StringVar(&packerLog, "packer_log", "packer.log", "The machine readable log of the packer build.")
    flag.StringVar(&cloudformation, "cloudform", "cloudformation.json", "The cloudformation template json.")
}

type TemplateData struct {
    Mappings string
}

type AMIMapping struct {
    AMI string
}

func main() {
    flag.Parse()
    logfile, err := os.OpenFile(packerLog, os.O_RDWR|os.O_CREATE, 0)
    if err != nil {
        panic(err)
    }
    c := csv.NewReader(logfile)
    c.LazyQuotes = true
    c.FieldsPerRecord = -1
    records, err := c.ReadAll()
    if err != nil {
        panic(err)
    }
    mappings := make(map[string]AMIMapping)
    for _, record := range records {
        if record[2] == "artifact" && record[4] == "id" {
            ids := strings.Split(record[5], ":")
            mappings[ids[0]] = AMIMapping{ids[1]}
        }
    }
    data := TemplateData{}
    mapString, err := json.Marshal(mappings)
    data.Mappings = string(mapString)
    if err != nil {
        panic(err)
    }
    tmpl, err := template.New("cloudformation.json").ParseFiles(cloudformation)
    if err != nil {
        panic(err)
    }
    err = tmpl.Execute(os.Stdout, data)
    if err != nil {
        panic(err)
    }
}
