import {
  buildPackagePowerShell,
  buildPackageRunBat,
  packageDownloadDirectory,
} from './downloadPackageRunner.js'

export interface MixedMediaItem {
  Id?: string
  Url: string
  Description?: string
  UrlSign?: string
  Suffix?: string
  Classify?: string
  DecodeStr?: string
  OtherData?: Record<string, string>
}

export interface MixedBatch {
  items: MixedMediaItem[]
  taskCount: number
}

export interface GenerateIdmMixedBatOptions {
  items: MixedMediaItem[]
  saveDirectory: string
  batchNum: number
  totalBatches: number
  filenameTime?: boolean
  buildFileName?: (item: MixedMediaItem) => string
}

export interface IdmPackageFile {
  path: string
  content: string
}

export interface IdmTaskPackage {
  name: string
  files: IdmPackageFile[]
}

export interface GenerateIdmTaskPackageOptions extends GenerateIdmMixedBatOptions {
  mixed: boolean
  quality?: number
}

interface PackageDownload {
  url: string
  originalUrl: string
  downloadDirectory: string
  directory: string
  filename: string
  tempFilename: string
  label: string
  encrypted: boolean
  decodeStr: string
}

interface PackageImageSet {
  folder: string
  metadataPath: string
  metadata: Record<string, unknown>
  images: Array<{
    url: string
    filename: string
  }>
  audio?: {
    url: string
    fileName: string
    name: string
  }
}

const idmPath = 'C:\\Program Files (x86)\\Internet Download Manager\\IDMan.exe'
const metadataChunkSize = 3000

interface DecryptTask {
  filePath: string
  decodeStr: string
  label: string
}

export const countMixedIdmTasks = (item: MixedMediaItem): number => {
  if (item.Classify !== 'image_set') {
    return 1
  }
  return Math.max(parseImageSetUrls(item).length, 1)
}

export const chunkMixedItems = (items: MixedMediaItem[], batchSize: number): MixedBatch[] => {
  const batches: MixedBatch[] = []
  let current: MixedMediaItem[] = []
  let currentCount = 0

  items.forEach(item => {
    const itemCount = countMixedIdmTasks(item)
    if (current.length > 0 && currentCount + itemCount > batchSize) {
      batches.push({items: current, taskCount: currentCount})
      current = []
      currentCount = 0
    }
    current.push(item)
    currentCount += itemCount
  })

  if (current.length > 0) {
    batches.push({items: current, taskCount: currentCount})
  }
  return batches
}

export const getMixedTaskCount = (items: MixedMediaItem[]): number => {
  return items.reduce((total, item) => total + countMixedIdmTasks(item), 0)
}

export const generateIdmTaskPackage = (options: GenerateIdmTaskPackageOptions): IdmTaskPackage => {
  const payload = buildTaskPayload(options)
  return {
    name: buildPackageName(options.batchNum),
    files: [
      {path: 'run.bat', content: buildPackageRunBat()},
      {path: 'idm_tasks.json', content: JSON.stringify(payload, null, 2)},
      {path: 'decrypt.ps1', content: buildPackagePowerShell()},
      {path: 'logs/.keep', content: ''},
    ],
  }
}

export const generateIdmMixedBat = (options: GenerateIdmMixedBatOptions): string => {
  const lines = buildBatHeader(options.batchNum, options.totalBatches)
  const decryptTasks: DecryptTask[] = []

  let imageSetIndex = 0
  options.items.forEach((item, index) => {
    lines.push(`echo(${escapeEchoText(`[${index + 1}/${options.items.length}] ${item.Description || item.UrlSign || item.Url}`)}`)
    if (item.Classify === 'image_set') {
      imageSetIndex += 1
      lines.push(...generateImageSetIdmCommands(item, imageSetIndex))
      return
    }
    const filename = options.buildFileName ? options.buildFileName(item) : defaultFileName(item)
    addIdmDownload(lines, item.Url, '%BASE_DIR%', filename)
    addDecryptTask(decryptTasks, item, `%BASE_DIR%\\${escapeBatArg(filename)}`, filename)
  })

  appendBatFooter(lines, decryptTasks)
  return lines.filter(line => line !== '').join('\r\n')
}

const buildTaskPayload = (options: GenerateIdmTaskPackageOptions) => {
  const downloads: PackageDownload[] = []
  const imageSets: PackageImageSet[] = []
  let imageSetIndex = 0

  options.items.forEach((item, index) => {
    if (options.mixed && item.Classify === 'image_set') {
      imageSetIndex += 1
      const imageSet = buildPackageImageSet(item, imageSetIndex)
      imageSets.push(imageSet)
      return
    }

    const filename = options.buildFileName ? options.buildFileName(item) : defaultFileName(item)
    const encrypted = Boolean(item.DecodeStr)
    downloads.push({
      url: applyWechatQualityUrl(item, options.quality || 0),
      originalUrl: item.Url,
      downloadDirectory: packageDownloadDirectory,
      directory: '.',
      filename,
      tempFilename: filename,
      label: item.Description || filename || item.Url,
      encrypted,
      decodeStr: item.DecodeStr || '',
    })
  })

  return {
    version: 1,
    app: 'res-downloader',
    batchNum: options.batchNum,
    totalBatches: options.totalBatches,
    mixed: options.mixed,
    createdAt: new Date().toISOString(),
    idmPath,
    downloads,
    imageSets,
  }
}

const buildPackageImageSet = (item: MixedMediaItem, exportIndex: number): PackageImageSet => {
  const urls = parseImageSetUrls(item)
  const setId = buildImageSetId(item, exportIndex)
  const folder = buildImageSetFolderName(item, setId)
  const images = urls.map((url, index) => ({
    url,
    filename: `${String(index + 1).padStart(3, '0')}${imageSuffixFromUrl(url)}`,
  }))
  const audio = buildImageSetAudio(item)
  const metadata = {
    type: 'wechat_channels_image_set',
    title: item.Description || '',
    description: item.OtherData?.image_set_description || '',
    topic: item.OtherData?.image_set_topic || '',
    source_url: item.OtherData?.image_set_source_url || '',
    publish_time: item.OtherData?.image_set_publish_time || '',
    original_id: item.OtherData?.image_set_original_id || '',
    set_id: setId,
    folder_name: folder,
    export_index: exportIndex,
    image_count: images.length,
    cover: images.length > 0 ? `images/${images[0].filename}` : '',
    images: images.map((image, index) => ({
      index: index + 1,
      file_name: image.filename,
      url: image.url,
      size: 0,
    })),
    ...(audio ? {
      audio: {
        file_name: audio.fileName,
        path: `audio/${audio.fileName}`,
        url: audio.url,
        size: 0,
        name: audio.name,
      }
    } : {}),
    captured_at: item.OtherData?.image_set_captured_at || new Date().toISOString()
  }

  return {
    folder,
    metadataPath: `${folder}\\metadata.json`,
    metadata,
    images,
    audio,
  }
}

const buildPackageName = (batchNum: number): string => {
  const suffix = batchNum > 0 ? `-${batchNum}` : ''
  return `res-downloader-${currentTimestamp()}${suffix}`
}

export const generateIdmBat = (options: GenerateIdmMixedBatOptions): string => {
  const lines = buildBatHeader(options.batchNum, options.totalBatches)
  const decryptTasks: DecryptTask[] = []

  options.items.forEach((item, index) => {
    const filename = options.buildFileName ? options.buildFileName(item) : defaultFileName(item)
    lines.push(`echo(${escapeEchoText(`[${index + 1}/${options.items.length}] ${filename}`)}`)
    addIdmDownload(lines, item.Url, '%BASE_DIR%', filename)
    addDecryptTask(decryptTasks, item, `%BASE_DIR%\\${escapeBatArg(filename)}`, filename)
  })

  appendBatFooter(lines, decryptTasks)
  return lines.filter(line => line !== '').join('\r\n')
}

const buildBatHeader = (batchNum: number, totalBatches: number): string[] => [
  '@echo off',
  'chcp 65001 >nul',
  `set "IDMAN=${idmPath}"`,
  'if not exist "%IDMAN%" set "IDMAN=C:\\Program Files\\Internet Download Manager\\IDMan.exe"',
  'if not exist "%IDMAN%" (',
  '  echo [错误] 未找到 IDMan.exe',
  '  pause',
  '  exit /b 1',
  ')',
  totalBatches > 1 ? `echo 批次 ${batchNum}/${totalBatches}` : '',
  'set "BASE_DIR=%~dp0"',
  'if "%BASE_DIR:~-1%"=="\\" set "BASE_DIR=%BASE_DIR:~0,-1%"',
  'echo 正在添加下载任务到 IDM...',
  'echo.'
]

const appendBatFooter = (lines: string[], decryptTasks: DecryptTask[]) => {
  lines.push('echo.')
  lines.push('"%IDMAN%" /s')
  lines.push('if errorlevel 1 (')
  lines.push('  echo [错误] IDM 启动下载队列失败')
  lines.push('  pause')
  lines.push('  exit /b 1')
  lines.push(')')
  lines.push('echo 所有任务已添加到 IDM')
  appendDecryptCommands(lines, decryptTasks)
  lines.push('pause')
}

export const getMaxLineLength = (content: string): number => {
  return content.split(/\r?\n/).reduce((max, line) => Math.max(max, line.length), 0)
}

const generateImageSetIdmCommands = (item: MixedMediaItem, exportIndex: number): string[] => {
  const urls = parseImageSetUrls(item)
  const setId = buildImageSetId(item, exportIndex)
  const folderName = buildImageSetFolderName(item, setId)
  const files = urls.map((url, index) => ({
    index: index + 1,
    file_name: `${String(index + 1).padStart(3, '0')}${imageSuffixFromUrl(url)}`,
    url,
    size: 0
  }))
  const audio = buildImageSetAudio(item)
  const metadata = {
    type: 'wechat_channels_image_set',
    title: item.Description || '',
    description: item.OtherData?.image_set_description || '',
    topic: item.OtherData?.image_set_topic || '',
    source_url: item.OtherData?.image_set_source_url || '',
    publish_time: item.OtherData?.image_set_publish_time || '',
    original_id: item.OtherData?.image_set_original_id || '',
    set_id: setId,
    folder_name: folderName,
    export_index: exportIndex,
    image_count: files.length,
    cover: files.length > 0 ? `images/${files[0].file_name}` : '',
    images: files,
    ...(audio ? {
      audio: {
        file_name: audio.fileName,
        path: `audio/${audio.fileName}`,
        url: audio.url,
        size: 0,
        name: audio.name,
      }
    } : {}),
    captured_at: item.OtherData?.image_set_captured_at || new Date().toISOString()
  }
  const metadataBase64 = base64Utf8(JSON.stringify(metadata, null, 2))
  const chunks = chunkString(metadataBase64, metadataChunkSize)
  const lines = [
    `set "SET_DIR=%BASE_DIR%\\${escapeBatPath(folderName)}"`,
    `set "IMG_DIR=%BASE_DIR%\\${escapeBatPath(folderName)}\\images"`,
    `set "META_JSON=%BASE_DIR%\\${escapeBatPath(folderName)}\\metadata.json"`,
    'if not exist "%IMG_DIR%" mkdir "%IMG_DIR%"',
    audio ? `set "AUDIO_DIR=%BASE_DIR%\\${escapeBatPath(folderName)}\\audio"` : '',
    audio ? 'if not exist "%AUDIO_DIR%" mkdir "%AUDIO_DIR%"' : '',
    'set "META_B64=%TEMP%\\resdownloader_meta_%RANDOM%_%RANDOM%.b64"',
    'type nul > "%META_B64%"'
  ]

  chunks.forEach(chunk => {
    lines.push(`>> "%META_B64%" echo ${chunk}`)
  })

  lines.push('powershell -NoProfile -ExecutionPolicy Bypass -Command "$b=[IO.File]::ReadAllText($env:META_B64); [IO.File]::WriteAllText($env:META_JSON,[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($b)),[Text.UTF8Encoding]::new($false))"')
  lines.push('if errorlevel 1 (')
  lines.push('  echo [错误] 写入图集 metadata.json 失败')
  lines.push('  pause')
  lines.push('  exit /b 1')
  lines.push(')')
  lines.push('if not exist "%META_JSON%" (')
  lines.push('  echo [错误] 图集 metadata.json 未生成')
  lines.push('  pause')
  lines.push('  exit /b 1')
  lines.push(')')
  lines.push('del "%META_B64%" >nul 2>nul')

  files.forEach(file => {
    addIdmDownload(lines, file.url, '%IMG_DIR%', file.file_name)
  })
  if (audio) {
    addIdmDownload(lines, audio.url, '%AUDIO_DIR%', audio.fileName)
  }

  return lines
}

const addIdmDownload = (lines: string[], url: string, directory: string, filename: string) => {
  lines.push(`"%IDMAN%" /d "${escapeBatArg(url)}" /p "${directory}" /f "${escapeBatArg(filename)}" /n /a`)
}

const addDecryptTask = (tasks: DecryptTask[], item: MixedMediaItem, filePath: string, label: string) => {
  if (!item.DecodeStr) {
    return
  }
  tasks.push({
    filePath,
    decodeStr: item.DecodeStr,
    label
  })
}

const appendDecryptCommands = (lines: string[], tasks: DecryptTask[]) => {
  if (tasks.length === 0) {
    return
  }

  lines.push('echo.')
  lines.push(`echo 检测到 ${tasks.length} 个微信加密视频。`)
  lines.push('echo 请等待 IDM 全部下载完成后，再回到本窗口继续解密。')
  lines.push('pause')
  lines.push('set "DECRYPT_PS1=%TEMP%\\resdownloader_decrypt_%RANDOM%_%RANDOM%.ps1"')
  lines.push('type nul > "%DECRYPT_PS1%"')
  lines.push('>> "%DECRYPT_PS1%" echo param([string]$File,[string]$KeyB64)')
  lines.push('>> "%DECRYPT_PS1%" echo $ErrorActionPreference = "Stop"')
  lines.push('>> "%DECRYPT_PS1%" echo if (!(Test-Path -LiteralPath $File)) { Write-Host "[skip] missing file: $File"; exit 2 }')
  lines.push('>> "%DECRYPT_PS1%" echo try {')
  lines.push('>> "%DECRYPT_PS1%" echo $key = [Convert]::FromBase64String($KeyB64)')
  lines.push('>> "%DECRYPT_PS1%" echo $fs = [IO.File]::Open($File,[IO.FileMode]::Open,[IO.FileAccess]::ReadWrite,[IO.FileShare]::Read)')
  lines.push('>> "%DECRYPT_PS1%" echo try {')
  lines.push('>> "%DECRYPT_PS1%" echo   $buffer = New-Object byte[] $key.Length')
  lines.push('>> "%DECRYPT_PS1%" echo   $read = $fs.Read($buffer,0,$buffer.Length)')
  lines.push('>> "%DECRYPT_PS1%" echo   for ($i = 0; $i -lt $read; $i++) { $buffer[$i] = $buffer[$i] -bxor $key[$i] }')
  lines.push('>> "%DECRYPT_PS1%" echo   $fs.Seek(0,[IO.SeekOrigin]::Begin) ^> $null')
  lines.push('>> "%DECRYPT_PS1%" echo   $fs.Write($buffer,0,$read)')
  lines.push('>> "%DECRYPT_PS1%" echo } finally {')
  lines.push('>> "%DECRYPT_PS1%" echo   $fs.Close()')
  lines.push('>> "%DECRYPT_PS1%" echo }')
  lines.push('>> "%DECRYPT_PS1%" echo } catch {')
  lines.push('>> "%DECRYPT_PS1%" echo   Write-Host "[error] decrypt failed: $_"')
  lines.push('>> "%DECRYPT_PS1%" echo   exit 3')
  lines.push('>> "%DECRYPT_PS1%" echo }')
  lines.push('>> "%DECRYPT_PS1%" echo exit 0')
  lines.push('echo 开始解密...')

  tasks.forEach((task, index) => {
    lines.push(`echo(${escapeEchoText(`[${index + 1}/${tasks.length}] ${task.label}`)}`)
    lines.push(`powershell -NoProfile -ExecutionPolicy Bypass -File "%DECRYPT_PS1%" "${task.filePath}" "${escapeBatArg(task.decodeStr)}"`)
    lines.push('if errorlevel 3 (')
    lines.push('  echo [错误] 解密执行失败')
    lines.push('  pause')
    lines.push('  exit /b 1')
    lines.push(')')
  })

  lines.push('del "%DECRYPT_PS1%" >nul 2>nul')
  lines.push('echo 解密完成')
}

const parseImageSetUrls = (item: MixedMediaItem): string[] => {
  try {
    const urls = JSON.parse(item.OtherData?.image_set_urls || '[]')
    return Array.isArray(urls) ? urls.filter(url => typeof url === 'string' && url) : []
  } catch (e) {
    return []
  }
}

const buildImageSetFolderName = (item: MixedMediaItem, setId: string): string => {
  const rawName = item.Description || item.UrlSign || 'image_set'
  const safeName = sanitizeWindowsName(rawName).slice(0, 70) || 'image_set'
  return `${safeName}_${setId}`
}

const buildImageSetId = (item: MixedMediaItem, exportIndex: number): string => {
  return `set${String(exportIndex).padStart(3, '0')}_${shortHash([
    item.Id || '',
    item.UrlSign || '',
    item.Description || '',
    item.OtherData?.image_set_urls || '',
  ].join('|'))}`
}

const buildImageSetAudio = (item: MixedMediaItem): PackageImageSet['audio'] => {
  const url = [
    item.OtherData?.image_set_audio_url,
    item.OtherData?.image_set_music_url,
    item.OtherData?.image_set_bgm_url,
  ].find(value => value && value.trim())
  if (!url) {
    return undefined
  }

  const rawFileName = item.OtherData?.image_set_audio_file_name || fileNameFromUrl(url) || `bgm${audioSuffixFromUrl(url)}`
  let fileName = sanitizeWindowsName(rawFileName) || `bgm${audioSuffixFromUrl(url)}`
  if (!/\.[a-z0-9]+$/i.test(fileName)) {
    fileName += audioSuffixFromUrl(url)
  }

  return {
    url,
    fileName,
    name: item.OtherData?.image_set_audio_name || '',
  }
}

const defaultFileName = (item: MixedMediaItem): string => {
  const name = sanitizeWindowsName(item.Description || item.UrlSign || 'video') || 'video'
  const suffix = item.Suffix || '.mp4'
  return name.endsWith(suffix) ? name : `${name}${suffix}`
}

const applyWechatQualityUrl = (item: MixedMediaItem, quality: number): string => {
  const rawUrl = item.Url || ''
  if (!rawUrl.includes('qq.com')) {
    return rawUrl
  }

  try {
    const parsedUrl = new URL(rawUrl)
    if (quality === 1 && parsedUrl.searchParams.has('encfilekey') && parsedUrl.searchParams.has('token')) {
      return `${parsedUrl.protocol}//${parsedUrl.host}${parsedUrl.pathname}?encfilekey=${parsedUrl.searchParams.get('encfilekey')}&token=${parsedUrl.searchParams.get('token')}`
    }

    const formats = (item.OtherData?.wx_file_formats || '').split('#').filter(Boolean)
    if (quality > 1 && formats.length > 0) {
      const qualityMap = [
        formats[0],
        formats[Math.floor(formats.length / 2)],
        formats[formats.length - 1],
      ]
      const format = qualityMap[quality - 2]
      if (format) {
        parsedUrl.searchParams.set('X-snsvideoflag', format)
        return parsedUrl.toString()
      }
    }
  } catch (e) {
    return rawUrl
  }

  return rawUrl
}

const sanitizeWindowsName = (value: string): string => {
  return value
      .replace(/[<>:"/\\|?*\r\n\t]/g, '_')
      .replace(/\s+/g, ' ')
      .replace(/[. ]+$/g, '')
      .trim()
}

const currentTimestamp = (): string => {
  const now = new Date()
  return [
    now.getFullYear(),
    String(now.getMonth() + 1).padStart(2, '0'),
    String(now.getDate()).padStart(2, '0'),
    String(now.getHours()).padStart(2, '0'),
    String(now.getMinutes()).padStart(2, '0'),
    String(now.getSeconds()).padStart(2, '0')
  ].join('')
}

const imageSuffixFromUrl = (rawUrl: string): string => {
  try {
    const ext = new URL(rawUrl).pathname.match(/\.(jpg|jpeg|png|webp|gif|bmp|avif)$/i)?.[0]
    return ext ? ext.toLowerCase() : '.jpg'
  } catch (e) {
    return '.jpg'
  }
}

const audioSuffixFromUrl = (rawUrl: string): string => {
  try {
    const ext = new URL(rawUrl).pathname.match(/\.(mp3|m4a|aac|wav|ogg|flac|amr|mp4)$/i)?.[0]
    return ext ? ext.toLowerCase() : '.m4a'
  } catch (e) {
    return '.m4a'
  }
}

const fileNameFromUrl = (rawUrl: string): string => {
  try {
    const pathname = new URL(rawUrl).pathname
    const last = pathname.split('/').filter(Boolean).pop() || ''
    return sanitizeWindowsName(last)
  } catch (e) {
    return ''
  }
}

const base64Utf8 = (value: string): string => {
  const bytes = new TextEncoder().encode(value)
  let binary = ''
  bytes.forEach(byte => binary += String.fromCharCode(byte))
  return btoa(binary)
}

const chunkString = (value: string, size: number): string[] => {
  const chunks: string[] = []
  for (let i = 0; i < value.length; i += size) {
    chunks.push(value.slice(i, i + size))
  }
  return chunks
}

const shortHash = (value: string): string => {
  let hash = 2166136261
  for (let i = 0; i < value.length; i++) {
    hash ^= value.charCodeAt(i)
    hash = Math.imul(hash, 16777619)
  }
  return (hash >>> 0).toString(16).padStart(8, '0').slice(0, 8)
}

const escapeBatArg = (value: string): string => {
  return String(value || '').replace(/"/g, '').replace(/%/g, '%%')
}

const escapeBatPath = (value: string): string => {
  return escapeBatArg(value).replace(/[<>|?*]/g, '_')
}

const escapeEchoText = (value: string): string => {
  return escapeBatArg(value)
      .replace(/\^/g, '^^')
      .replace(/&/g, '^&')
      .replace(/\|/g, '^|')
      .replace(/</g, '^<')
      .replace(/>/g, '^>')
}
