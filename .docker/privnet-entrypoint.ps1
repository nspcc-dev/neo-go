#!C:\Program Files\PowerShell\7\pwsh.EXE -File

$bin = '/usr/bin/neo-go.exe'

for ( $i = 0; $i -lt $args.count; $i++ ) {
	if ($args[$i] -eq "node"){
		Write-Host "=> Try to restore blocks before running node"
		if (($Env:ACC -ne $null) -and (Test-Path $Env:ACC -PathType Leaf)) {
			& $bin db restore -p --config-path /config -i $Env:ACC
		}
		break
	}
}

& $bin $args
