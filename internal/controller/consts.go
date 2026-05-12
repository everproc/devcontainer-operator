package controller

import "os"

var (
	GIT_IMAGE_NAME       string
	PARSERAPP_IMAGE_NAME string
	KANIKO_IMAGE_NAME    string
)

func init() {
	GIT_IMAGE_NAME = os.Getenv("GIT_IMAGE_NAME")
	if GIT_IMAGE_NAME == "" {
		GIT_IMAGE_NAME = "ghcr.io/everproc/git-clone:0.0.10"
	}

	PARSERAPP_IMAGE_NAME = os.Getenv("PARSERAPP_IMAGE_NAME")
	if PARSERAPP_IMAGE_NAME == "" {
		PARSERAPP_IMAGE_NAME = "ghcr.io/everproc/parserapp:0.0.10"
	}

	KANIKO_IMAGE_NAME = os.Getenv("KANIKO_IMAGE_NAME")
	if KANIKO_IMAGE_NAME == "" {
		KANIKO_IMAGE_NAME = "everproc/kaniko-executor:v1.25.14"
	}
}
