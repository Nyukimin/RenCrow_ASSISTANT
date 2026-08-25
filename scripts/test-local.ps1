param(
    [string[]]$Step = @(),
    [switch]$KeepRuntime,
    [switch]$SelfTest
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
$planPath = Join-Path $PSScriptRoot "test-local.plan.json"
$runtimeRoot = Join-Path $repoRoot "Tmp\test-runtime"
$runRoot = Join-Path $runtimeRoot ("r\{0}-{1}" -f $PID, ([Guid]::NewGuid().ToString("N").Substring(0, 8)))
$cacheRoot = Join-Path $runtimeRoot "cache"

if (-not (Test-Path -LiteralPath $planPath -PathType Leaf)) {
    throw "Canonical test plan does not exist: $planPath"
}
$plan = Get-Content -LiteralPath $planPath -Raw | ConvertFrom-Json
if ($plan.version -ne 1) {
    throw "Unsupported canonical test plan version: $($plan.version)"
}

$commands = @()
$names = @()
foreach ($planStep in @($plan.steps)) {
    $name = [string]$planStep.name
    if ([string]::IsNullOrWhiteSpace($name) -or $names -contains $name) {
        throw "Canonical test step names must be non-empty and unique: $name"
    }
    $names += $name
    if ($Step.Count -gt 0 -and $Step -notcontains $name) {
        continue
    }
    $commands += [pscustomobject]@{
        Name = $name
        WorkingDirectory = [string]$planStep.workingDirectory
        FilePath = [string]$planStep.filePath
        Arguments = @($planStep.arguments | ForEach-Object { [string]$_ })
    }
}
foreach ($requested in $Step) {
    if ($names -notcontains $requested) {
        throw "Unknown canonical test step: $requested"
    }
}
if ($commands.Count -eq 0) {
    throw "Canonical test plan contains no selected steps"
}

$paths = @{
    TEMP = $runRoot
    TMP = $runRoot
    TMPDIR = $runRoot
    GOTMPDIR = (Join-Path $runRoot "go-build")
    GOCACHE = (Join-Path $cacheRoot "go-build")
    GOMODCACHE = (Join-Path $cacheRoot "go-mod")
}
$previous = @{}
foreach ($name in $paths.Keys) {
    $previous[$name] = [Environment]::GetEnvironmentVariable($name, "Process")
    New-Item -ItemType Directory -Force -Path $paths[$name] | Out-Null
    [Environment]::SetEnvironmentVariable($name, $paths[$name], "Process")
}

try {
    Write-Host "[test-local] repository: $repoRoot"
    Write-Host "[test-local] runtime: $runRoot"
    Write-Host "[test-local] plan: $planPath"
    Write-Host "[test-local] steps: $($commands.Name -join ', ')"

    if ($SelfTest) {
        foreach ($name in $paths.Keys) {
            $value = [Environment]::GetEnvironmentVariable($name, "Process")
            if (-not ([IO.Path]::GetFullPath($value)).StartsWith([IO.Path]::GetFullPath($runtimeRoot))) {
                throw "$name escaped the repository-local test runtime: $value"
            }
        }
        Write-Host "[OK] Repository-local test runtime and canonical plan contract passed"
        return
    }

    foreach ($command in $commands) {
        $workingPath = [IO.Path]::GetFullPath((Join-Path $repoRoot $command.WorkingDirectory))
        if (-not $workingPath.StartsWith($repoRoot)) {
            throw "WorkingDirectory must stay inside the repository: $workingPath"
        }
        Write-Host "[test-local] step: $($command.Name)"
        Push-Location $workingPath
        try {
            $global:LASTEXITCODE = 0
            & $command.FilePath @($command.Arguments)
            if ($LASTEXITCODE -ne 0) {
                throw "Test step '$($command.Name)' failed with exit code $LASTEXITCODE"
            }
        } finally {
            Pop-Location
        }
    }
} finally {
    foreach ($name in $paths.Keys) {
        [Environment]::SetEnvironmentVariable($name, $previous[$name], "Process")
    }
    if (-not $KeepRuntime -and (Test-Path -LiteralPath $runRoot)) {
        Remove-Item -LiteralPath $runRoot -Recurse -Force
    }
}
