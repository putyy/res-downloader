' Notes: wanted to implement this using a class but:
' 1. No matter what I did I could not assign the result of GetObject to a private member
' 2. It looks as if all methods were treated as subs from the outside world which is not good since 
' some of these need to return a value

' should be removed when migration is complete
Set private_oReg = GetObject("winmgmts:\root\default:StdRegProv")		

Set private_oCtx = CreateObject("WbemScripting.SWbemNamedValueSet")
private_oCtx.Add "__ProviderArchitecture", CInt(OSArchitecture)

Set private_oLocator = CreateObject("Wbemscripting.SWbemLocator")
Set private_oServices = private_oLocator.ConnectServer(".", "root\default","","",,,,private_oCtx)
Set private_oRegSpecific = private_oServices.Get("StdRegProv") 

Function CheckAccess(hDefKey,sSubKeyName,uRequired, bGranted )	
	Set Inparams = private_oRegSpecific.Methods_("CheckAccess").Inparameters
	
	Inparams.hDefKey = hDefKey
	
	Inparams.sSubKeyName = sSubKeyName
	
	Inparams.uRequired = uRequired
		
	set Outparams = private_oRegSpecific.ExecMethod_("CheckAccess", Inparams,,private_oCtx)
	
	bGranted = Outparams.bGranted
	

	CheckAccess = 0
	
End Function

Function CreateKey(hDefKey,sSubKeyName)	
	Set Inparams = private_oRegSpecific.Methods_("CreateKey").Inparameters
	
	Inparams.hDefKey = hDefKey
	
	Inparams.sSubKeyName = sSubKeyName
		
	set Outparams = private_oRegSpecific.ExecMethod_("CreateKey", Inparams,,private_oCtx)
	

	CreateKey = 0
	
End Function

Function DeleteKey(hDefKey,sSubKeyName)	
	Set Inparams = private_oRegSpecific.Methods_("DeleteKey").Inparameters
	
	Inparams.hDefKey = hDefKey
	
	Inparams.sSubKeyName = sSubKeyName
		
	set Outparams = private_oRegSpecific.ExecMethod_("DeleteKey", Inparams,,private_oCtx)
	

	DeleteKey = 0
	
End Function

Function DeleteValue(hDefKey,sSubKeyName,sValueName)	
	Set Inparams = private_oRegSpecific.Methods_("DeleteValue").Inparameters
	
	Inparams.hDefKey = hDefKey
	
	Inparams.sSubKeyName = sSubKeyName
	
	Inparams.sValueName = sValueName
		
	set Outparams = private_oRegSpecific.ExecMethod_("DeleteValue", Inparams,,private_oCtx)
	

	DeleteValue = 0
	
End Function

Function EnumKey(hDefKey,sSubKeyName, sNames )	
	Set Inparams = private_oRegSpecific.Methods_("EnumKey").Inparameters
	
	Inparams.hDefKey = hDefKey
	
	Inparams.sSubKeyName = sSubKeyName
		
	set Outparams = private_oRegSpecific.ExecMethod_("EnumKey", Inparams,,private_oCtx)
	
	sNames = Outparams.sNames
	

	EnumKey = 0
	
End Function

Function EnumValues(hDefKey,sSubKeyName, sNames,Types )	
	Set Inparams = private_oRegSpecific.Methods_("EnumValues").Inparameters
	
	Inparams.hDefKey = hDefKey
	
	Inparams.sSubKeyName = sSubKeyName
		
	set Outparams = private_oRegSpecific.ExecMethod_("EnumValues", Inparams,,private_oCtx)
	
	sNames = Outparams.sNames
	
	Types = Outparams.Types
	

	EnumValues = 0
	
End Function

Function GetBinaryValue(hDefKey,sSubKeyName,sValueName, uValue )	
	Set Inparams = private_oRegSpecific.Methods_("GetBinaryValue").Inparameters
	
	Inparams.hDefKey = hDefKey
	
	Inparams.sSubKeyName = sSubKeyName
	
	Inparams.sValueName = sValueName
		
	set Outparams = private_oRegSpecific.ExecMethod_("GetBinaryValue", Inparams,,private_oCtx)
	
	uValue = Outparams.uValue
	

	GetBinaryValue = 0
	
End Function

Function GetDWORDValue(hDefKey,sSubKeyName,sValueName, uValue )	
	Set Inparams = private_oRegSpecific.Methods_("GetDWORDValue").Inparameters
	
	Inparams.hDefKey = hDefKey
	
	Inparams.sSubKeyName = sSubKeyName
	
	Inparams.sValueName = sValueName
		
	set Outparams = private_oRegSpecific.ExecMethod_("GetDWORDValue", Inparams,,private_oCtx)
	
	uValue = Outparams.uValue
	

	GetDWORDValue = 0
	
End Function

Function GetExpandedStringValue(hDefKey,sSubKeyName,sValueName, sValue )	
	Set Inparams = private_oRegSpecific.Methods_("GetExpandedStringValue").Inparameters
	
	Inparams.hDefKey = hDefKey
	
	Inparams.sSubKeyName = sSubKeyName
	
	Inparams.sValueName = sValueName
		
	set Outparams = private_oRegSpecific.ExecMethod_("GetExpandedStringValue", Inparams,,private_oCtx)
	
	sValue = Outparams.sValue
	

	GetExpandedStringValue = 0
	
End Function

Function GetMultiStringValue(hDefKey,sSubKeyName,sValueName, sValue )	
	Set Inparams = private_oRegSpecific.Methods_("GetMultiStringValue").Inparameters
	
	Inparams.hDefKey = hDefKey
	
	Inparams.sSubKeyName = sSubKeyName
	
	Inparams.sValueName = sValueName
		
	set Outparams = private_oRegSpecific.ExecMethod_("GetMultiStringValue", Inparams,,private_oCtx)
	
	sValue = Outparams.sValue
	

	GetMultiStringValue = 0
	
End Function

Function GetQWORDValue(hDefKey,sSubKeyName,sValueName, uValue )	
	Set Inparams = private_oRegSpecific.Methods_("GetQWORDValue").Inparameters
	
	Inparams.hDefKey = hDefKey
	
	Inparams.sSubKeyName = sSubKeyName
	
	Inparams.sValueName = sValueName
		
	set Outparams = private_oRegSpecific.ExecMethod_("GetQWORDValue", Inparams,,private_oCtx)
	
	uValue = Outparams.uValue
	

	GetQWORDValue = 0
	
End Function

Function GetSecurityDescriptor(hDefKey,sSubKeyName, Descriptor )	
	Set Inparams = private_oRegSpecific.Methods_("GetSecurityDescriptor").Inparameters
	
	Inparams.hDefKey = hDefKey
	
	Inparams.sSubKeyName = sSubKeyName
		
	set Outparams = private_oRegSpecific.ExecMethod_("GetSecurityDescriptor", Inparams,,private_oCtx)
	
	Descriptor = Outparams.Descriptor
	

	GetSecurityDescriptor = 0
	
End Function

Function GetStringValue(hDefKey,sSubKeyName,sValueName, sValue )	
	Set Inparams = private_oRegSpecific.Methods_("GetStringValue").Inparameters
	
	Inparams.hDefKey = hDefKey
	
	Inparams.sSubKeyName = sSubKeyName
	
	Inparams.sValueName = sValueName
		
	set Outparams = private_oRegSpecific.ExecMethod_("GetStringValue", Inparams,,private_oCtx)
	
	sValue = Outparams.sValue
	

	GetStringValue = 0
	
End Function

Function SetBinaryValue(hDefKey,sSubKeyName,sValueName,uValue)	
	Set Inparams = private_oRegSpecific.Methods_("SetBinaryValue").Inparameters
	
	Inparams.hDefKey = hDefKey
	
	Inparams.sSubKeyName = sSubKeyName
	
	Inparams.sValueName = sValueName
	
	Inparams.uValue = uValue
		
	set Outparams = private_oRegSpecific.ExecMethod_("SetBinaryValue", Inparams,,private_oCtx)
	

	SetBinaryValue = 0
	
End Function

Function SetDWORDValue(hDefKey,sSubKeyName,sValueName,uValue)	
	Set Inparams = private_oRegSpecific.Methods_("SetDWORDValue").Inparameters
	
	Inparams.hDefKey = hDefKey
	
	Inparams.sSubKeyName = sSubKeyName
	
	Inparams.sValueName = sValueName
	
	Inparams.uValue = uValue
		
	set Outparams = private_oRegSpecific.ExecMethod_("SetDWORDValue", Inparams,,private_oCtx)
	

	SetDWORDValue = 0
	
End Function

Function SetExpandedStringValue(hDefKey,sSubKeyName,sValueName,sValue)	
	Set Inparams = private_oRegSpecific.Methods_("SetExpandedStringValue").Inparameters
	
	Inparams.hDefKey = hDefKey
	
	Inparams.sSubKeyName = sSubKeyName
	
	Inparams.sValueName = sValueName
	
	Inparams.sValue = sValue
		
	set Outparams = private_oRegSpecific.ExecMethod_("SetExpandedStringValue", Inparams,,private_oCtx)
	

	SetExpandedStringValue = 0
	
End Function

Function SetMultiStringValue(hDefKey,sSubKeyName,sValueName,sValue)	
	Set Inparams = private_oRegSpecific.Methods_("SetMultiStringValue").Inparameters
	
	Inparams.hDefKey = hDefKey
	
	Inparams.sSubKeyName = sSubKeyName
	
	Inparams.sValueName = sValueName
	
	Inparams.sValue = sValue
		
	set Outparams = private_oRegSpecific.ExecMethod_("SetMultiStringValue", Inparams,,private_oCtx)
	

	SetMultiStringValue = 0
	
End Function

Function SetQWORDValue(hDefKey,sSubKeyName,sValueName,uValue)	
	Set Inparams = private_oRegSpecific.Methods_("SetQWORDValue").Inparameters
	
	Inparams.hDefKey = hDefKey
	
	Inparams.sSubKeyName = sSubKeyName
	
	Inparams.sValueName = sValueName
	
	Inparams.uValue = uValue
		
	set Outparams = private_oRegSpecific.ExecMethod_("SetQWORDValue", Inparams,,private_oCtx)
	

	SetQWORDValue = 0
	
End Function

Function SetSecurityDescriptor(hDefKey,sSubKeyName,Descriptor)	
	Set Inparams = private_oRegSpecific.Methods_("SetSecurityDescriptor").Inparameters
	
	Inparams.hDefKey = hDefKey
	
	Inparams.sSubKeyName = sSubKeyName
	
	Inparams.Descriptor = Descriptor
		
	set Outparams = private_oRegSpecific.ExecMethod_("SetSecurityDescriptor", Inparams,,private_oCtx)
	

	SetSecurityDescriptor = 0
	
End Function

Function SetStringValue(hDefKey,sSubKeyName,sValueName,sValue)	
	Set Inparams = private_oRegSpecific.Methods_("SetStringValue").Inparameters
	
	Inparams.hDefKey = hDefKey
	
	Inparams.sSubKeyName = sSubKeyName
	
	Inparams.sValueName = sValueName
	
	Inparams.sValue = sValue
		
	set Outparams = private_oRegSpecific.ExecMethod_("SetStringValue", Inparams,,private_oCtx)
	

	SetStringValue = 0
	
End Function
