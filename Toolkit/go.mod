module github.com/superagent/toolkit

go 1.21

require (
	github.com/HelixDevelopment/HelixAgent-SiliconFlow v0.0.0
	github.com/HelixDevelopment/HelixAgent-Chutes v0.0.0
)

replace (
	github.com/HelixDevelopment/HelixAgent-SiliconFlow => ./SiliconFlow
	github.com/HelixDevelopment/HelixAgent-Chutes => ./Toolkit/Chutes
)