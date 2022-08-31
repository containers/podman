# Windows Installer Build

## Requirements

1. Win 10+
2. Golang
3. MingW
4. Dotnet SDK (if AzureSignTool)
5. AzureSignTool (optional)
6. WiX Toolset

## Usage

```
.\build.ps1 <version> [prod|dev] [release_dir]
```

## One off build (-dev output (default), unsigned (default))

```
.\build.ps1 4.2.0
```

## Build with a pre-downloaded win release zip in my-download dir

```
.\build.ps1 4.2.0 dev my-download
```
