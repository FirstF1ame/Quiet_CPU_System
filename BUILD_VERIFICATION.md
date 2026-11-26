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

사용자가 제공한 [VirusTotal 탐지 화면](docs/virustotal-detection.png)에 다음 탐지 3건이 표시됩니다. 화면 상단의 위협 분류는 `trojan.generic`입니다.

| 탐지 엔진 | 표시 결과 |
| --- | --- |
| Bkav Pro | `W32.Malware.C293072F` |
| Elastic | `Malicious (moderate Confidence)` |
| McAfee Scanner | `Trojan:Win/GenericY.HVE` |

Microsoft, Bitdefender, Kaspersky 등은 화면에서 `Undetected`로 표시됩니다. Arctic Wolf는 파일 형식을 처리하지 못했다고 표시합니다.

제공된 화면에는 **검사 파일의 SHA-256, 파일명, 전체 엔진 수, 검사일**이 보이지 않습니다. 따라서 어떤 아키텍처의 배포 EXE에 이 결과가 해당하는지 아직 확인되지 않았습니다. 위 재빌드 결과는 배포 파일과 공개 소스의 일치 여부를 검증한 기록입니다. 오탐 여부를 판단하려면 원본 보고서의 SHA-256을 위 표와 대조한 뒤 해당 파일의 분석 결과를 확인해야 합니다.

[공식 VirusTotal 해시 검색 안내](https://docs.virustotal.com/docs/searching) · [VirusTotal 탐지 결과 및 오탐 안내](https://docs.virustotal.com/docs/false-positive)
