#Requires -Version 5.1

<#
.SYNOPSIS
    Generates Markdown documentation for Shoutrrr services.

.DESCRIPTION
    This script generates Markdown documentation for Shoutrrr services using the shoutrrr CLI.
    It supports generating documentation for all services or a specific service.
    Skips 'standard' and 'xmpp' services as they are not applicable.

.PARAMETER ServiceName
    Optional. The name of a specific service to generate documentation for.
    If not provided, generates documentation for all services.

.EXAMPLE
    .\generate-service-config-docs.ps1

    Generates documentation for all services.

.EXAMPLE
    .\generate-service-config-docs.ps1 -ServiceName discord

    Generates documentation for the discord service only.
#>

param(
    [string]$ServiceName
)

# Enable strict error handling
$ErrorActionPreference = "Stop"

# Get script and repository paths
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$repoRoot = Split-Path -Parent $scriptDir
$servicesPath = Join-Path $repoRoot "pkg\services"

# Function to generate documentation for a service
function Generate-Docs {
    param(
        [string]$Service,
        [string]$Category
    )

    $docsPath = Join-Path $repoRoot "docs\services\$Category\$Service"

    Write-Host "Creating docs for $Category/$Service... " -ForegroundColor Cyan -NoNewline

    # Create directory if it doesn't exist
    New-Item -ItemType Directory -Path $docsPath -Force | Out-Null

    # Run shoutrrr docs command
    $shoutrrrPath = Join-Path $repoRoot "shoutrrr"
    $outputFile = Join-Path $docsPath "config.md"

    $process = Start-Process -FilePath "go" -ArgumentList "run", "`"$shoutrrrPath`"", "docs", "-f", "markdown", $Service -RedirectStandardOutput $outputFile -NoNewWindow -Wait -PassThru

    if ($process.ExitCode -eq 0) {
        Write-Host "Done!" -ForegroundColor Green
    } else {
        Write-Host "Failed!" -ForegroundColor Red
    }
}

# Check if a specific service was requested
if ($ServiceName) {
    # Find the service directory
    $serviceDir = Get-ChildItem -Path $servicesPath -Recurse -Directory | Where-Object { $_.Name -eq $ServiceName } | Select-Object -First 1

    if (-not $serviceDir) {
        Write-Host "Service $ServiceName not found" -ForegroundColor Red
        exit 1
    }

    $category = Split-Path -Parent $serviceDir.FullName | Split-Path -Leaf
    Generate-Docs -Service $ServiceName -Category $category
    exit 0
}

# Debug: Print the services path
Write-Host "Debug: Checking services path: $servicesPath"

# Check if services directory exists and has subdirectories
if (-not (Test-Path $servicesPath)) {
    Write-Host "Services path $servicesPath does not exist" -ForegroundColor Red
    exit 1
}

$categoryDirs = Get-ChildItem -Path $servicesPath -Directory
if ($categoryDirs.Count -eq 0) {
    Write-Host "No service directories found in $servicesPath" -ForegroundColor Red
    Write-Host "Debug: Contents of ${servicesPath}:"
    Get-ChildItem -Path $servicesPath | Format-Table -AutoSize
    exit 1
}

# Process all services
foreach ($categoryDir in $categoryDirs) {
    $category = $categoryDir.Name

    $serviceDirs = Get-ChildItem -Path $categoryDir.FullName -Directory

    foreach ($serviceDir in $serviceDirs) {
        $service = $serviceDir.Name

        # Skip specific services
        if ($service -eq "standard" -or $service -eq "xmpp") {
            continue
        }

        # Debug: Print the service being processed
        Write-Host "Debug: Processing service: $category/$service"

        Generate-Docs -Service $service -Category $category
    }
}