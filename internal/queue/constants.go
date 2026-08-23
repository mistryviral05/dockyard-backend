package queue

const (
	nixpacksImage = "dockyard/nixpacks:1.41.0"
)

// Sirf wahi entries jo build ke liye kabhi zaroori nahi hoti. dist/build aur
// .env jaan boojh kar chhode hain: kuch repos apna build output commit karte
// hain, unko hataya to app hi na chale.
const defaultDockerignore = `.git
.gitignore
node_modules
__pycache__
*.pyc
.venv
venv
*.log
.DS_Store
.vscode
.idea
`