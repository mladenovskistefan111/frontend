package main

import (
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
)

var packagingServiceUrl string

type PackagingInfo struct {
    Weight float32 `json:"weight"`
    Width  float32 `json:"width"`
    Height float32 `json:"height"`
    Depth  float32 `json:"depth"`
}

func init() {
    packagingServiceUrl = os.Getenv("PACKAGING_SERVICE_URL")
}

func isPackagingServiceConfigured() bool {
    return packagingServiceUrl != ""
}

func httpGetPackagingInfo(productId string) (*PackagingInfo, error) {
    url := packagingServiceUrl + "/" + productId
    fmt.Println("Requesting packaging info from URL: ", url)
    resp, err := http.Get(url)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
    }
    responseBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }
    var packagingInfo PackagingInfo
    err = json.Unmarshal(responseBody, &packagingInfo)
    return &packagingInfo, err
}