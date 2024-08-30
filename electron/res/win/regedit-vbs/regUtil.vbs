' TODO: consider incorporating a json writer of some sort instead of adhoc solution like the following
' e.g: http://demon.tw/my-work/vbs-json.html

const HKEY_CLASSES_ROOT = &H80000000
const HKEY_CURRENT_USER = &H80000001
const HKEY_LOCAL_MACHINE = &H80000002
const HKEY_USERS = &H80000003
const HKEY_CURRENT_CONFIG = &H80000005

Sub LoadRegistryImplementationByOSArchitecture()
	If IsNull(OSArchitecture) Then
		WriteLineErr "missing OSArchitecture global. did not call util.DetermineOSArchitecture? or Forgot to load util.vbs?"
		WScript.Quit 25125		
	End If

	If OSArchitecture = "A" Then
		Include "ArchitectureAgnosticRegistry.vbs"
	Else
		Include "ArchitectureSpecificRegistry.vbs"
	End If
End Sub 

Function PutValue(constHive, strSubKey, strValueName, strValue, strType)
	Select Case UCase(strType)
		
		Case "REG_SZ"
			PutValue = SetStringValue(constHive, strSubKey, strValueName, strValue)

		Case "REG_EXPAND_SZ"
			PutValue = SetExpandedStringValue(constHive, strSubKey, strValueName, strValue)

		Case "REG_BINARY"
			PutValue = SetBinaryValue(constHive, strSubKey, strValueName, ToBinaryValue(strValue))

		Case "REG_NONE"
			PutValue = SetBinaryValue(constHive, strSubKey, strValueName, ToBinaryValue(strValue))

		' TODO: need to check that indeed int is the right type here
		Case "REG_DWORD"
			PutValue = SetDWORDValue(constHive, strSubKey, strValueName, CDbl(strValue))

		Case "REG_MULTI_SZ"
			PutValue = SetMultiStringValue(constHive, strSubKey, strValueName, Split(strValue, ","))

		Case "REG_QWORD"
			PutValue = SetQWORDValue(constHive, strSubKey, strValueName, strValue)

		Case "REG_DEFAULT"
			PutValue = SetStringValue(constHive, strSubKey, "", strValue)

		Case Else
			PutValue = SetStringValue(constHive, strSubKey, strValueName, strValue)

	End Select
End Function

' render the child of a sub path strSubKey in hive constHive
' as json.
Sub ListChildrenAsJson(constHive, strSubKey)
	' start outputting json to stdout
	Write "{"

	Dim e1: e1 = EnumKey (constHive, strSubKey, arrKeyNames)
	If e1 <> 0 Then
		Write """exists"": false,"
		Dim arrValueNames: arrValueNames = null
	Else
		Write """exists"": true,"

		Dim e2: e2 = EnumValues (constHive, strSubKey, arrValueNames, arrValueTypes)
		If e2 <> 0 Then
			WScript.Quit e2
		End If
	End If

	Write """keys"": ["
	If Not IsNull(arrKeyNames) Then
		For x = 0 To UBound(arrKeyNames)
			If (x > 0) Then
				Write ","
			End If
	
			Write """" & JsonSafe(arrKeyNames(x)) & """"
		Next
	End If
	Write "],"
	' TODO: some duplicity of code between the two paths of this condition, this needs to be address at some point
	Write """values"":{"
	If Not IsNull(arrValueNames) Then
		For y = 0 To UBound(arrValueNames)
			If y > 0 Then
				Write ","
			End If

			strValueName = arrValueNames(y)
			intValueType = arrValueTypes(y)
			
			' assign the value to varValue
			GetValueByType constHive, strSubKey, strValueName, intValueType, varValue
			
			WriteValue strValueName, intValueType, varValue
		Next
	Else
		' fix for keys with only default values in them
		' see http://stackoverflow.com/questions/8840343/how-to-read-the-default-value-from-registry-in-vbscript
		GetStringValue constHive, strSubKey, "", strDefaultValue

		If IsNull(strDefaultValue) = false and strDefaultValue <> "" Then
			' write the default value with REG_SZ
			WriteValue "", 1, strDefaultValue
		End If
	End If
	Write "}}"	
End Sub

Sub WriteValue (strValueName, intValueType, varValue)
	Write """"  
	Write JsonSafe(strValueName)
	Write """:{"
	Write """type"": """
	Write RenderType(intValueType)
	Write ""","
	Write """value"":"
	Write RenderValueByType(intValueType, varValue)
	Write "}"
End Sub

' give a raw HKLM\something\somewhere
' output the hive constant and the subkey, in this case:
' HKEY_LOCAL_MACHINE will be assigned to outConstHive
' and something\somewhere will be assigned to outStrSubKey
Sub ParseHiveAndSubKey(strRawKey, outConstHive, outStrSubKey)	
	' split into two parts to deduce the hive and the sub key
	arrSplitted = Split(strRawKey, "\", 2, 1)
	
	If UBound(arrSplitted) > 0 Then
		strHive = arrSplitted(0)	
		outStrSubKey = arrSplitted(1)
	Else
		strHive = strRawKey
		outStrSubKey = ""
	End If

	outConstHive = StringToHiveConst(UCase(strHive))
End Sub

Function ArrayRemoveAt(arr, pos)
	Dim i
	If IsArray(arr) Then
		If pos >= 0 And pos <= UBound(arr) Then
			For i = pos To UBound(arr) - 1
				arr(i) = arr(i + 1)
			Next
			ReDim Preserve arr(UBound(arr) - 1)
		End If
	End If
End Function

Sub ParseHiveAndSubKeyAndValue(strRawKey, outConstHive, outStrSubKey, outStrValue)
	' split into two parts to deduce the hive and the sub key
	arrSplitted = Split(strRawKey, "\", -1, 1)
  
  If UBound(arrSplitted) > 0 Then
		strHive = arrSplitted(0)
		outStrValue = arrSplitted(UBound(arrSplitted))
		test = ArrayRemoveAt(arrSplitted, UBound(arrSplitted))
		test = ArrayRemoveAt(arrSplitted, 0)
		outStrSubKey = Join(arrSplitted, "\")
	Else
		strHive = strRawKey
		outStrSubKey = ""
	End If

	outConstHive = StringToHiveConst(UCase(strHive))
End Sub

Function StringToHiveConst(strHive)
	
	Select Case strHive
		Case "HKCR"
			StringToHiveConst = HKEY_CLASSES_ROOT
		Case "HKCU"
			StringToHiveConst = HKEY_CURRENT_USER
		Case "HKLM"
			StringToHiveConst = HKEY_LOCAL_MACHINE
		Case "HKU"
			StringToHiveConst = HKEY_USERS
		Case "HKCC"
			StringToHiveConst = HKEY_CURRENT_CONFIG
		Case Else
			StringToHiveConst = Null	
	End Select	

End Function

' TODO: this entire "by type" should be transformed into OOP style
' where each type will have a class with render(), getValue() etc...

' convert a value type number into a string label
Function RenderType(intType)
	RenderType = "REG_UNKNOWN"

	Select Case intType
		Case 0
			RenderType = "REG_NONE"
		Case 1
			RenderType = "REG_SZ"
		Case 2
			RenderType = "REG_EXPAND_SZ"
		Case 3
			RenderType = "REG_BINARY"
		Case 4
			RenderType = "REG_DWORD"
		Case 7
			RenderType = "REG_MULTI_SZ"
		Case 11	
			RenderType = "REG_QWORD"
		Case Else
			' TODO: should report / throw an error here
			WriteErr("invalid Registry Value Type " & intType)

	End Select

End Function

' render by value type:
' string will return as a string with double quotes, e.g "value"
' multi string values which return as an array ot strings "["1", "2"]" (double quotes included ofc)
' numeric values like DWORD and QWORD just return as the number e.g. 1
' byte arrays such as reg_binary return as an array of ints, e.g [1,2,3]
Function RenderValueByType(intType, varValue)

	Select Case intType
		' REG_NONE
		Case 0
			RenderValueByType = "0"

		' REG_SZ
		Case 1
			RenderValueByType = """" & JsonSafe(varValue) & """"

		' REG_EXPAND_SZ
		Case 2
			RenderValueByType = """" & JsonSafe(varValue) & """"

		' REG_BINARY
		Case 3
			RenderValueByType = RenderByteArray(varValue)

		' REG_DWORD
		Case 4
			RenderValueByType= varValue

		' REG_MULYI_SZ'
		Case 7

			RenderValueByType = RenderStringArray(varValue)
		' REG_QWORD
		Case 11
			RenderValueByType = varValue
		Case Else
			' TODO: should report / throw an error here
			WriteErr("invalid Registry Value Type " & intType)
	End Select

End Function

' get the value of a registry based on its value type and assign it to out parameter outVarValue
Sub GetValueByType(constHive, strKey, strValueName, intType, outVarValue)

	Select Case intType
		' REG_NONE
		Case 0
			GetStringValue constHive, strKey, strValueName, "0"
			Exit Sub

		' REG_SZ
		Case 1
			GetStringValue constHive, strKey, strValueName, outVarValue
			Exit Sub

		' REG_EXPAND_SZ
		Case 2
			GetExpandedStringValue constHive, strKey, strValueName, outVarValue			
			Exit Sub
			
		' REG_BINARY
		Case 3
			GetBinaryValue constHive, strKey, strValueName, outVarValue
			Exit Sub
			
		' REG_DWORD
		Case 4
			GetDWORDValue constHive, strKey, strValueName, outVarValue

			' #21 - VBS does not support UInt32. This is the workaround
			If outVarValue < 0 Then outVarValue = 4294967296 + outVarValue

			Exit Sub
			
		' REG_MULYI_SZ'
		Case 7
			GetMultiStringValue constHive, strKey, strValueName, outVarValue
			Exit Sub
			
		' REG_QWORD
		Case 11	
			GetQWORDValue constHive, strKey, strValueName, outVarValue
			Exit Sub
		
		Case Else
			' TODO: should report / throw an error here
			WriteErr("invalid Registry Value Type " & intType)	
	End Select

End Sub

' render a byte array as a json array of numbers
Function RenderByteArray(arr)
	RenderByteArray = "[]"

	If Not IsNull(arr) Then		
		RenderByteArray = "[" & Join(arr, ",") & "]"
	End If
End Function

' render a string array as json string array
Function RenderStringArray(arr)	
	Result = "["
	If Not IsNull(arr) Then
		For t = 0 To UBound(arr)
			If (t > 0) Then
				Result = Result &  ","
			End If

			Result = Result & """" & JsonSafe(arr(t)) & """"
		Next
	End If
	Result = Result & "]"

	RenderStringArray = Result
End Function

Function ToBinaryValue(strValue)

	arrValue = Split(strValue, ",")
	
	If IsNull(arrValue) Then		
		ToBinaryValue = Array()
		Exit Function
	End If

	For i = 0 To UBound(arrValue)
		arrValue(i) = CInt(arrValue(i))
	Next

	ToBinaryValue = arrValue
End Function