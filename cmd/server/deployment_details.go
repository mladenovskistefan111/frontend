package main

import (
    "os"

    "github.com/sirupsen/logrus"
)

var deploymentDetailsMap map[string]string
var log *logrus.Logger

func init() {
    log = logrus.New()
    log.Formatter = &logrus.JSONFormatter{
        FieldMap: logrus.FieldMap{
            logrus.FieldKeyTime:  "timestamp",
            logrus.FieldKeyLevel: "severity",
            logrus.FieldKeyMsg:   "message",
        },
    }
    log.Out = os.Stdout

    deploymentDetailsMap = make(map[string]string)
    if hostname, err := os.Hostname(); err == nil {
        deploymentDetailsMap["HOSTNAME"] = hostname
    }
}