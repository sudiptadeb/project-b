package instance

// Owner is nothing on Android, where the system runs one copy of the app.
type Owner struct{}

// Claim always owns the folder: Android starts no second copy.
func Claim(dir string, urls []string) (*Owner, error) { return &Owner{}, nil }

// Serve has nothing to serve.
func (o *Owner) Serve(handover func(urls []string), quit func()) {}

// Quit finds no other copy.
func Quit(dir string) (int, error) { return 0, nil }

// Close has nothing to give up.
func (o *Owner) Close() {}
