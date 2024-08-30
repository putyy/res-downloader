Set stdout = WScript.StdOut
Set stderr = WScript.StdErr
Set stdin = WScript.StdIn
Set args = WScript.Arguments
Set fs = CreateObject("scripting.filesystemobject") 
Dim OSArchitecture

Sub WriteErr(message)
	stderr.Write message
End Sub

Sub WriteLineErr(message)
	stderr.WriteLine message
End Sub

Sub Write(message)
	stdout.Write message
End Sub

Sub WriteLine(message)
	stdout.WriteLine message
End Sub

Function IndexOf(varNeedle, arrHaystack)
	IndexOf = -1
	
	If Not IsArray(arrHaystack)	 Then
		Exit Function
	End If

	For xyz = 0 To UBound(arrHaystack)
		If arrHaystack(xyz) = varNeedle Then
			IndexOf = xyz
			Exit Function
		End If
	Next
End Function

Sub CheckZeroArgs(message)
	' bail if args are missing
	If args.Count = 0 Then
		WriteLineErr message
		WScript.Quit 25121
	End If
End Sub

Dim ALLOWED_OS_ARCHITECTURE_VALUES: ALLOWED_OS_ARCHITECTURE_VALUES = Array("S", "A", "32", "64")

'
'	determine the architecture of the operating system, that will be used. there are 4 possibilities:
'	A - means agnostic
'	S - means that we want to use a specific architecture, but auto detect Item
'	32 - explicitly use 32 bit architecture
'	64 - explicitly use 64 bit architecture
'
Sub DetermineOSArchitecture()
	strArchitecture = args(0)

	If IsNull(strArchitecture) Then
		WriteLineErr "missing architecture argument"
		WScript.Quit 25124
	End If

	strArchitecture = UCase(strArchitecture)

	If IndexOf(strArchitecture, ALLOWED_OS_ARCHITECTURE_VALUES) = -1 Then
		WriteLineErr "invalid architecture argument"
		WScript.Quit 25124
	End If

	If (strArchitecture = "S") Then
		OSArchitecture = GetOSArchitecture()
		If OSArchitecture = -1 Then
			WriteLineErr "invalid os architecture detected " & OSArchitecture
			WScript.Quit 25126
		End If
	Else
		OSArchitecture = strArchitecture
	End If

End Sub

Sub Include(sPath)
	' TODO this is fragile, but should work for "modules" nested relatively to script root
	include_ScriptPath = Left(WScript.ScriptFullName, InStr(WScript.ScriptFullName, WScript.ScriptName) - 2)	
	sPath = include_ScriptPath & "\" & sPath
	
	include_code = fs.OpenTextFile(sPath).ReadAll 	
	ExecuteGlobal include_code
End Sub

Function GetOSArchitecture()

	Dim ObjWMI, ColSettings, ObjProcessor 
	Dim StrComputer, ObjNetwork 
	
	Set ObjWMI = GetObject("winmgmts:\Root\CIMV2") 
	Set ColSettings = ObjWMI.ExecQuery ("SELECT DataWidth, AddressWidth, Architecture FROM Win32_Processor") 

	' TODO: I make two assumptions here: 
	' 1. Eveyone will have CPU0 device
	' 2. There is only one cpu defined in the wmi database (and if not, then they are all of the same architecture)
	Set ObjProcessor = ColSettings.Item("Win32_Processor.DeviceID=""CPU0""")

	If ObjProcessor.Architecture = 0 AND ObjProcessor.AddressWidth = 32 Then 
		GetOSArchitecture = 32
	ElseIf (ObjProcessor.Architecture = 6 OR ObjProcessor.Architecture = 9) AND ObjProcessor.DataWidth = 64 AND ObjProcessor.AddressWidth = 32 Then 
		GetOSArchitecture = 32
	ElseIf (ObjProcessor.Architecture = 6 OR ObjProcessor.Architecture = 9) AND ObjProcessor.DataWidth = 64 AND ObjProcessor.AddressWidth = 64 Then 
		GetOSArchitecture = 64
	Else		
		GetOSArchitecture = -1
	End If 
	
End Function

Function JsonSafe(inStrText)
	If inStrText = "" Then
		JsonSafe = ""
		Exit Function 
	End If
	Dim outStrText: outStrText = inStrText
	outStrText = Replace(outStrText, "\", "\\")
	outStrText = Replace(outStrText, vbcrlf, "\\r\\n")
	outStrText = Replace(outStrText, vblf, "\\n")
	outStrText = Replace(outStrText, vbcr, "\\r")
	outStrText = Replace(outStrText, """", "\""")	
	outStrText = JsonU(outStrText)
	JsonSafe = outStrText
End Function

'TODO: need to change this function's name to something more appropriate
Function JsonU(astr)
	
	If isNull(astr) Then
		JsonU = ""
		Exit Function
	End If

	Dim c 
	Dim utftext: utftext = ""
	
	For n = 1 To Len(astr)
		c = CLng(AscW(Mid(astr, n, 1)))

		If c < 0 Then
			c = &H10000 + c
		End If

		If c < &H80 Then
			utftext = utftext & Mid(astr, n, 1)
		ElseIf c < &H100 Then
			utftext = utftext & "\u00" & Hex(c)
		ElseIf c < &H1000 Then
			utftext = utftext & "\u0" & Hex(c)
		Else
			utftext = utftext & "\u" & Hex(c)
		End If
	Next

	JsonU = utftext
End Function
