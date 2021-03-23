# Project 2


## Rabin-Karp Chunking
Before inserting two spaces:
![chunk1](./src/chunk1.png)

After adding two spaces:
![chunk2](./src/chunk2.png)

## Networking with http

put 
----
#### Putting a file to local server:
Recipes and blobs will be stored under `server_store` directory locally.

Client: The recipe for `testfile` is `sha256_32_BPL5QDZ5ABDB366ASD7XZ7IOYUUPBDJGDHYMCFEZSUNODYAWIQIA====`
![put_local_file](./src/put_local_file.png)


Server: 

![put_local_file2](./src/put_local_file2.png)

#### Putting a directory to local server:

Test directory structure:
![put_dir](./src/put_dir.png)

Put result:
![put_dir2](./src/put_dir2.png)
The recipe for `testdir` is `sha256_32_HAI2Q6VKTR4DJRNOFLGRXJRRJIIWJA637KPGZNUD22MIQSDPQLZA====`


get
----
#### Getting a blob from local server:
Pick a random blob `sha256_32_WNFQ76UQ23IMTJ34JFLAHMK2KNANNALLJBRQS543CME2I46OOCUQ====` and rename it to `datachunk`:
![get_blob](./src/get_blob.png)

#### Getting a file from local server:
Use the recipe `sha256_32_BPL5QDZ5ABDB366ASD7XZ7IOYUUPBDJGDHYMCFEZSUNODYAWIQIA====` for `testfile` to reconstruct the file:
![getfile_local](./src/getfile_local.png)

#### Getting a directory from local server:
Use the recipe `sha256_32_HAI2Q6VKTR4DJRNOFLGRXJRRJIIWJA637KPGZNUD22MIQSDPQLZA====` for `testdir` to reconstruct the directory:
![getdir_local](./src/getdir_local.png)

#### Error Handling
Trying to `get` from a non-existing signature:
![404_get](./src/404_get.png)

getfile
----

When running a `get` command on a file recipe signature, `get` will call `getfile` internally. So behaviors are the same for `get` and `getfile` when the signature file is a file recipe.

#### Getting a file from local server:
Use the recipe `sha256_32_BPL5QDZ5ABDB366ASD7XZ7IOYUUPBDJGDHYMCFEZSUNODYAWIQIA====` for `testfile` to reconstruct `newfile`:
![getfile_file](./src/getfile_file.png)

#### Error Handling
Trying to use `getfile` on a non-existing signature:
![404_getfile](./src/404_getfile.png)

Trying to use `getfile` on a directory recipe signature:
![500_getfile](./src/500_getfile.png)

desc
----
#### `desc` on a file sig
![desc-file](./src/desc-file.png)

#### `desc` on a directory sig
![desc-dir](./src/desc-dir.png)

#### `desc` on a blob sig
![desc-blob](./src/desc-blob.png)

## Remote server

`put` to and `get` from remote server:
![remote_put](./src/remote_put.png)
![remote_get](./src/remote_get.png)

`desc`:
![remote_desc](./src/remote_desc.png)

## Extra credit
The `cats` directory contains cat images.

1. `put` the directory to the local server and get the signature for the directory recipe: `sha256_32_3DPW4JUKUHNW7FJI5JJAXCZRRC6OCZVXNKCHQ6PDR4FPK6SWHAVQ====`
![cat](./src/cat.png)

2. Video demo:
https://youtu.be/TEJr83Gtn9E
[![Watch the video](./src/videodemo.png)](https://youtu.be/TEJr83Gtn9E)
 

