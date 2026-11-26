# Windows 실행 파일 검증

2026년 11월 26일(KST)에 [커밋 `ab71b26`](https://github.com/FirstF1ame/Quiet_CPU_System/tree/ab71b269ae41b566ab1a5b10c62a74204020e36c)의 소스와 배포 파일을 확인했습니다.

## SHA-256과 재빌드 결과

| 실행 파일 | SHA-256 | 재빌드 비교 |
| --- | --- | --- |
| [Quiet_Session_Windows_x64.exe](dist/Quiet_Session_Windows_x64.exe) | `cef3c453da3c252522d42ca093d23fa118cc0cb6c4c0e83d440520aff29b3986` | 바이트 단위 일치 |
| [Quiet_Session_Windows_ARM64.exe](dist/Quiet_Session_Windows_ARM64.exe) | `5a9073eceb081bf7e7b8acd1a1cd09a7f20e3a99dd9ba5323a9a6172d6efade3` | 바이트 단위 일치 |
| [Quiet_Session_Windows_x86.exe](dist/Quiet_Session_Windows_x86.exe) | `7b92a62699b5a1f35b045e35df011ebed0891c300bf7850b1a95502f6788e560` | 바이트 단위 일치 |

Go 1.25.5에서 README의 빌드 옵션을 사용했습니다. Windows 대상 아키텍처 `amd64`, `arm64`, `386`으로 각각 빌드한 결과가 해당 커밋에 포함된 EXE와 바이트 단위로 일치했습니다. `go test ./...`도 통과했습니다.

다운로드한 EXE의 SHA-256은 PowerShell에서 확인할 수 있습니다. 예시는 x64 파일입니다.

```powershell
Get-FileHash .\Quiet_Session_Windows_x64.exe -Algorithm SHA256
```

## VirusTotal 검사 상태

**미확인.** 이 검증 시점에는 VirusTotal 웹 업로드 화면에 접속하지 못해 검사 보고서와 탐지 수치를 확보하지 못했습니다. 위의 해시와 재빌드 결과는 배포 파일이 공개 소스와 일치한다는 증거입니다. 악성코드 검사 여부는 VirusTotal 보고서로 별도 확인해야 하며, 탐지 수치는 해당 시점에 사용된 엔진의 결과입니다.

[공식 VirusTotal 해시 검색 안내](https://docs.virustotal.com/docs/searching)
