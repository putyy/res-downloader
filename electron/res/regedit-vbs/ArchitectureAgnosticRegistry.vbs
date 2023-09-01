' Notes: wanted to implement this using a class but:
' 1. No matter what I did I could not assign the result of GetObject to a private member
' 2. It looks as if all methods were treated as subs from the outside world which is not good since 
' some of these need to return a value
'

Set private_oReg = GetObject("winmgmts:\root\default:StdRegProv")		

Function SetStringValue(constHive, strSubKey, strValueName, strValue)	
	SetStringValue  = private_oReg.SetStringValue(constHive, strSubKey, strValueName, strValue)	
End Function

Sub GetStringValue(constHive, strKey, strValueName, strValue)
	private_oReg.GetStringValue constHive, strKey, strValueName, strValue
End Sub

Function SetExpandedStringValue(constHive, strSubKey, strValueName, strValue)
	SetExpandedStringValue = private_oReg.SetExpandedStringValue(constHive, strSubKey, strValueName, strValue)
End Function

Sub GetExpandedStringValue(constHive, strKey, strValueName, strValue)
	private_oReg.GetExpandedStringValue constHive, strKey, strValueName, strValue
End Sub

Function SetMultiStringValue(constHive, strSubKey, strValueName, arrValue)
	SetMultiStringValue = private_oReg.SetMultiStringValue(constHive, strSubKey, strValueName, arrValue)
End Function

Sub GetMultiStringValue(constHive, strKey, strValueName, arrStrValue)
	private_oReg.GetMultiStringValue constHive, strKey, strValueName, arrStrValue
End Sub 

Function SetDWORDValue(constHive, strSubKey, strValueName, arrValue)
	SetDWORDValue = private_oReg.SetDWORDValue(constHive, strSubKey, strValueName, arrValue)
End Function

Sub GetDWORDValue(constHive, strKey, strValueName, intDWordValue)
	private_oReg.GetDWORDValue constHive, strKey, strValueName, intDWordValue
End Sub

Function SetQWORDValue(constHive, strSubKey, strValueName, strQWordValue)
	SetQWORDValue = private_oReg.SetQWORDValue(constHive, strSubKey, strValueName, strQWordValue)
End Function

Sub GetQWORDValue(constHive, strKey, strValueName, intQWordValue)
	private_oReg.GetQWORDValue constHive, strKey, strValueName, intQWordValue
End Sub

Function SetBinaryValue(constHive, strSubKey, strValueName, arrValue)
	SetBinaryValue = private_oReg.SetBinaryValue(constHive, strSubKey, strValueName, arrValue)
End Function

Sub GetBinaryValue(constHive, strKey, strValueName, arrBinaryValue)
	private_oReg.GetBinaryValue constHive, strKey, strValueName, arrBinaryValue
End Sub

Function EnumKey(constHive, strSubKey, arrKeyNames)
	EnumKey = private_oReg.EnumKey(constHive, strSubKey, arrKeyNames)
End Function

Function EnumValues(constHive, strSubKey, arrValueNames, arrValueTypes)
	EnumValues = private_oReg.EnumValues(constHive, strSubKey, arrValueNames, arrValueTypes)
End Function

Function CreateKey(constHive, strSubKey)
	CreateKey = private_oReg.CreateKey(constHive, strSubKey)
End Function

Function DeleteKey(constHive, strSubKey)
	DeleteKey = private_oReg.DeleteKey(constHive, strSubKey)
End Function

Function DeleteValue(constHive, strSubKey, strValue)
	DeleteValue = private_oReg.DeleteValue(constHive, strSubKey, strValue)
End Function
