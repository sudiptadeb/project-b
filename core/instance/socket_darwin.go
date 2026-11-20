package instance

import "os"

// socketDir is the user's temporary folder, which every copy of the app shares.
func socketDir() string { return os.TempDir() }
