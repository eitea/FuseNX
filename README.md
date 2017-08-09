# FuseNX
BackupGUI

## Creating the executable

You need the [go-bindata-assetfs](https://github.com/elazarl/go-bindata-assetfs) to create the bindata file.

    go-bindata-assetfs data/...
    go build

You can rebuild the syso file, but it's usually not necessary

    rsrc -manifest FuseNX.exe.manifest -ico icon.ico -o FuseNX.syso