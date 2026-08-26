package queue

import "time"

const (
	nixpacksImage = "dockyard/nixpacks:1.41.0"
)

const (
	// logBatchSize: ek publish me kitni lines. History 500 messages ki hai,
	// isliye 20 lines per message ~10,000 lines cover karta hai.
	logBatchSize = 20

	// logFlushInterval: itni der chup rehne pe adhoora batch bhi bhej do,
	// warna slow build me screen jami hui lagti hai.
	logFlushInterval = 400 * time.Millisecond

	// logBufferSize: buffer bhar gaya to lines girti hain. Build ko rokne se
	// behtar hai kuch lines kho jayein.
	logBufferSize = 2048

	logPublishTimeout = 5 * time.Second

	// DB ke liye alag aur bada batch: browser ko lines turant chahiye,
	// disk ko nahi. Live wala har 400ms/20 lines pe jaata hai, ye har
	// 1s/50 lines pe, taki ek build me insert bahut kam baar ho.
	logStoreBatchSize     = 50
	logStoreFlushInterval = time.Second
	logStoreTimeout       = 5 * time.Second
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