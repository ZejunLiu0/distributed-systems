go run client.go -s localhost:8880 -q key.private anchor zliu1238
go run client.go -s localhost:8880 desc last
go run client.go -s localhost:8880 put status.html
go run client.go -s localhost:8880 -q fake.private claim zliu1238 rootsig last
go run client.go -s localhost:8880 put status.html
go run client.go -s localhost:8880 -q key.private claim zliu1238 rootsig last
go run client.go -s localhost:8880 desc last

go run client.go -s localhost:8880 -q fake.private anchor zliu1238
go run client.go -s localhost:8880 -q key.private anchor zliu1238



