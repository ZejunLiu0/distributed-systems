go run client.go -s localhost:8880 put status.html
go run client.go -s localhost:8880 desc last
go run client.go -s localhost:8880 -q key.private sign last
go run client.go -s localhost:8880 desc last
go run client.go -s localhost:8880 -p key.public verify last
go run client.go -s localhost:8880 -p fake.public verify last
go run client.go -s localhost:8880 -p key.public verify last