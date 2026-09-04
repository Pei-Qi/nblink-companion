$ErrorActionPreference = "Stop"
$RootDir = Resolve-Path (Join-Path $PSScriptRoot "..")
$Version = if ($env:VERSION) { $env:VERSION } else { "0.3.0" }
$BuildNumber = if ($env:BUILD_NUMBER) { $env:BUILD_NUMBER } else { "1" }
$Arch = if ($env:GOARCH) { $env:GOARCH } else { "amd64" }
$WailsTool = "github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.16"
$BuildDir = Join-Path $RootDir "dist\windows\$Arch"
$AssetDir = Join-Path $RootDir "dist\assets"
$IconPng = Join-Path $AssetDir "NblinkCompanion-1024.png"
$IconIco = Join-Path $AssetDir "NblinkCompanion.ico"
$ManifestPath = Join-Path $BuildDir "nblink-companion.exe.manifest"
$InfoPath = Join-Path $BuildDir "info.json"
$SysoPath = Join-Path $RootDir "cmd\nblink-companion\rsrc_windows_$Arch.syso"
$ReleaseExe = Join-Path $BuildDir "Nblink-Companion-$Version-windows-$Arch.exe"
$Utf8NoBom = New-Object System.Text.UTF8Encoding($false)

$env:GOOS = "windows"
$env:GOARCH = $Arch

Push-Location $RootDir
try {
    New-Item -ItemType Directory -Force -Path $BuildDir, $AssetDir | Out-Null

    if ($env:SKIP_FRONTEND_BUILD -ne "1") {
        Push-Location (Join-Path $RootDir "frontend")
        try {
            npm ci
            npm run build
        }
        finally {
            Pop-Location
        }
    }

    go run .\cmd\iconbuilder `
        -svg .\assets\app-icon.svg `
        -png $IconPng `
        -ico $IconIco

    go run .\cmd\iconbuilder `
        -svg .\assets\tray-icon.svg `
        -png .\assets\tray-icon.png `
        -png-size 64

    $Manifest = @"
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly manifestVersion="1.0" xmlns="urn:schemas-microsoft-com:asm.v1" xmlns:asmv3="urn:schemas-microsoft-com:asm.v3">
  <assemblyIdentity type="win32" name="com.local.nblink-companion" version="$Version.$BuildNumber" processorArchitecture="*"/>
  <dependency>
    <dependentAssembly>
      <assemblyIdentity type="win32" name="Microsoft.Windows.Common-Controls" version="6.0.0.0" processorArchitecture="*" publicKeyToken="6595b64144ccf1df" language="*"/>
    </dependentAssembly>
  </dependency>
  <asmv3:application>
    <asmv3:windowsSettings>
      <dpiAware xmlns="http://schemas.microsoft.com/SMI/2005/WindowsSettings">true/pm</dpiAware>
      <dpiAwareness xmlns="http://schemas.microsoft.com/SMI/2016/WindowsSettings">permonitorv2,permonitor</dpiAwareness>
    </asmv3:windowsSettings>
  </asmv3:application>
</assembly>
"@
    [System.IO.File]::WriteAllText($ManifestPath, $Manifest, $Utf8NoBom)

    $Info = @{
        fixed = @{ file_version = "$Version.$BuildNumber" }
        info = @{
            "0000" = @{
                ProductVersion = $Version
                FileDescription = "节点小宝固定端口伴侣"
                ProductName = "Nblink Companion"
                Comments = "Wails desktop companion for Nblink fixed-port forwarding"
            }
        }
    } | ConvertTo-Json -Depth 4
    [System.IO.File]::WriteAllText($InfoPath, $Info, $Utf8NoBom)

    Remove-Item -Force -ErrorAction SilentlyContinue $SysoPath, $ReleaseExe
    go run $WailsTool generate syso `
        -arch $Arch `
        -icon $IconIco `
        -manifest $ManifestPath `
        -info $InfoPath `
        -out $SysoPath

    go build -trimpath -tags production -ldflags "-s -w -H windowsgui" `
        -o $ReleaseExe .\cmd\nblink-companion

    if (-not (Test-Path $ReleaseExe)) {
        throw "Go build did not create $ReleaseExe"
    }
    Write-Host "Created $ReleaseExe"
}
finally {
    Remove-Item -Force -ErrorAction SilentlyContinue $SysoPath
    Pop-Location
}
