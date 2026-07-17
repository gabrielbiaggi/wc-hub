package terraform

// Runner executes pinned Terraform images in isolated workers. The API process
// must never invoke terraform directly or inherit cloud credentials.
type Runner struct {
	image         string
	workspaceRoot string
}

func New(image, workspaceRoot string) *Runner {
	return &Runner{image: image, workspaceRoot: workspaceRoot}
}
