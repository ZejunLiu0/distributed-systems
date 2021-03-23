go run client.go del localhost:8880 all
go run client.go del localhost:8881 all
go run client.go info localhost:8880
go run client.go info localhost:8881
go run client.go put localhost:8880 ../../p1/samplefile
go run client.go put localhost:8881 ../../p1/sampledir
go run client.go sync localhost:8880 localhost:8881 3
go run client.go sync localhost:8881 localhost:8880 3
go run client.go sync localhost:8880 localhost:8881 3
go run client.go sync localhost:8881 localhost:8880 3
rm -fr xd
go run client.go get localhost:8880 last xd
diff xd ../../p1/sampledir
go run client.go del localhost:8880 all
go run client.go del localhost:8881 all
go run client.go put localhost:8880 ../../p1/samplefile
go run client.go put localhost:8881 ../../p1/sampledir
go run client.go sync localhost:8880 localhost:8881 1
go run client.go sync localhost:8881 localhost:8880 1
go run client.go sync localhost:8880 localhost:8881 1
go run client.go sync localhost:8881 localhost:8880 1

