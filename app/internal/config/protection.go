package config

import "os"

// StorageProtected is a deployment assertion set only after host-volume
// encryption has been verified outside the container or by the macOS launcher.
func StorageProtected() bool { return os.Getenv("TGTLDR_STORAGE_PROTECTED") == "true" }
