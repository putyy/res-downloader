import assert from 'node:assert/strict'
import {test} from 'node:test'
import {
  chunkMixedItems,
  generateIdmBat,
  generateIdmMixedBat,
  generateIdmTaskPackage,
  getMaxLineLength,
} from '../tmp-test/idmMixed.js'

const longUrl = `https://finder.video.qq.com/251/20304/stodownload?token=${'a'.repeat(900)}`

const videoItem = (id) => ({
  Id: `video-${id}`,
  Url: `${longUrl}-${id}`,
  Description: `video ${id}`,
  UrlSign: `video-${id}`,
  Suffix: '.mp4',
  Classify: 'video',
  OtherData: {},
})

const wechatVideoItem = (id) => ({
  ...videoItem(id),
  Url: `https://finder.video.qq.com/251/20302/stodownload?encfilekey=key-${id}&token=token-${id}&svrnonce=1779116965`,
  OtherData: {
    wx_file_formats: 'low-format#mid-format#high-format',
  },
})

const encryptedVideoItem = (id) => ({
  ...videoItem(id),
  DecodeStr: 'AQIDBA==',
})

const imageSetItem = (id, count) => ({
  Id: `set-${id}`,
  Url: '',
  Description: `image set ${id}`,
  UrlSign: `set-${id}`,
  Suffix: '.jpg',
  Classify: 'image_set',
  OtherData: {
    image_set_description: 'long description '.repeat(120),
    image_set_urls: JSON.stringify(Array.from({length: count}, (_, index) => `${longUrl}-${id}-${index}`)),
    image_set_topic: 'topic',
    image_set_captured_at: '2026-05-17T09:00:00+08:00',
  },
})

const imageSetWithAudioItem = (id, count) => ({
  ...imageSetItem(id, count),
  OtherData: {
    ...imageSetItem(id, count).OtherData,
    image_set_audio_url: 'https://wx.example.com/bgm.m4a',
    image_set_audio_name: '背景音乐',
  },
})

test('mixed BAT keeps every generated line below cmd safe length', () => {
  const bat = generateIdmMixedBat({
    items: [imageSetItem('a', 14), videoItem('a')],
    saveDirectory: 'G:\\下载总\\res-downloader',
    batchNum: 1,
    totalBatches: 1,
  })

  assert.ok(getMaxLineLength(bat) < 4000)
  assert.match(bat, /META_B64/)
  assert.doesNotMatch(bat, /FromBase64String\('[A-Za-z0-9+/=]{8000,}'\)/)
})

test('mixed export batching counts image files as IDM tasks without splitting an image set', () => {
  const batches = chunkMixedItems([
    ...Array.from({length: 95}, (_, index) => videoItem(index)),
    imageSetItem('large', 14),
    ...Array.from({length: 3}, (_, index) => videoItem(`tail-${index}`)),
  ], 100)

  assert.equal(batches.length, 2)
  assert.equal(batches[0].items.length, 95)
  assert.equal(batches[0].taskCount, 95)
  assert.equal(batches[1].items.length, 4)
  assert.equal(batches[1].taskCount, 17)
  assert.equal(batches[1].items[0].Id, 'set-large')
})

test('mixed BAT downloads videos and image sets under the BAT directory', () => {
  const bat = generateIdmMixedBat({
    items: [videoItem('root'), imageSetItem('images', 2)],
    saveDirectory: 'G:\\下载总\\res-downloader',
    batchNum: 1,
    totalBatches: 1,
  })

  assert.match(bat, /set "BASE_DIR=%~dp0"/)
  assert.doesNotMatch(bat, /set "BASE_DIR=G:\\下载总\\res-downloader"/)
  assert.match(bat, /\/p "%BASE_DIR%" \/f "video root\.mp4"/)
})

test('mixed BAT carries image set audio into the direct export path', () => {
  const bat = generateIdmMixedBat({
    items: [imageSetWithAudioItem('audio', 2)],
    saveDirectory: 'G:\\下载总\\res-downloader',
    batchNum: 1,
    totalBatches: 1,
  })

  assert.match(bat, /set "AUDIO_DIR=%BASE_DIR%\\[^"]+\\audio"/)
  assert.match(bat, /\/p "%AUDIO_DIR%" \/f "bgm\.m4a"/)
})

test('image set folders use the title first like video filenames', () => {
  const pack = generateIdmTaskPackage({
    items: [imageSetItem('named', 2)],
    saveDirectory: 'G:\\下载总\\res-downloader',
    batchNum: 1,
    totalBatches: 1,
    mixed: true,
  })
  const tasks = JSON.parse(pack.files.find(file => file.path === 'idm_tasks.json').content)
  const folder = tasks.imageSets[0].folder

  assert.match(folder, /^image set named_set001_[a-f0-9]{8}$/)
  assert.doesNotMatch(folder, /^\d{14}_set001_/)
})

test('ordinary IDM BAT downloads to the BAT directory and starts the IDM queue', () => {
  const bat = generateIdmBat({
    items: [videoItem('plain')],
    saveDirectory: 'G:\\下载总\\res-downloader',
    batchNum: 1,
    totalBatches: 1,
  })

  assert.match(bat, /set "BASE_DIR=%~dp0"/)
  assert.match(bat, /\/p "%BASE_DIR%" \/f "video plain\.mp4" \/n \/a/)
  assert.match(bat, /"%IDMAN%" \/s/)
})

test('BAT decrypts encrypted WeChat videos after IDM finishes', () => {
  const pack = generateIdmTaskPackage({
    items: [encryptedVideoItem('secret'), videoItem('plain')],
    saveDirectory: 'G:\\下载总\\res-downloader',
    batchNum: 1,
    totalBatches: 1,
    mixed: false,
  })
  const runBat = pack.files.find(file => file.path === 'run.bat').content
  const tasksJson = pack.files.find(file => file.path === 'idm_tasks.json').content
  const decryptPs1 = pack.files.find(file => file.path === 'decrypt.ps1').content
  const tasks = JSON.parse(tasksJson)

  assert.ok(getMaxLineLength(runBat) < 4000)
  assert.doesNotMatch(runBat, /AQIDBA==/)
  assert.match(runBat, /decrypt\.ps1/)
  assert.match(decryptPs1, /FromBase64String\(\$task\.decodeStr\)/)
  assert.match(decryptPs1, /-bxor/)
  assert.equal(tasks.downloads.length, 2)
  assert.equal(tasks.downloads[0].encrypted, true)
  assert.equal(tasks.downloads[0].decodeStr, 'AQIDBA==')
  assert.equal(tasks.downloads[1].encrypted, false)
  assert.equal(tasks.downloads[1].decodeStr, '')
})

test('task package keeps a huge decode key out of BAT command lines', () => {
  const pack = generateIdmTaskPackage({
    items: [{
      ...encryptedVideoItem('huge'),
      DecodeStr: 'A'.repeat(180000),
    }],
    saveDirectory: 'G:\\下载总\\res-downloader',
    batchNum: 1,
    totalBatches: 1,
    mixed: false,
  })
  const runBat = pack.files.find(file => file.path === 'run.bat').content
  const tasksJson = pack.files.find(file => file.path === 'idm_tasks.json').content

  assert.ok(getMaxLineLength(runBat) < 4000)
  assert.ok(tasksJson.includes('A'.repeat(180000)))
  assert.doesNotMatch(runBat, /A{1000}/)
})

test('task package does not abort when IDM add returns a non-zero code', () => {
  const pack = generateIdmTaskPackage({
    items: [encryptedVideoItem('secret')],
    saveDirectory: 'G:\\下载总\\res-downloader',
    batchNum: 1,
    totalBatches: 1,
    mixed: false,
  })
  const decryptPs1 = pack.files.find(file => file.path === 'decrypt.ps1').content

  assert.doesNotMatch(decryptPs1, /throw \("IDM add failed:/)
  assert.match(decryptPs1, /if \(\$null -ne \$LASTEXITCODE -and \$LASTEXITCODE -ne 0\) \{ Log \("IDM add returned code "/)
})

test('task package does not abort when IDM queue start returns a non-zero code', () => {
  const pack = generateIdmTaskPackage({
    items: [encryptedVideoItem('secret')],
    saveDirectory: 'G:\\下载总\\res-downloader',
    batchNum: 1,
    totalBatches: 1,
    mixed: false,
  })
  const decryptPs1 = pack.files.find(file => file.path === 'decrypt.ps1').content

  assert.doesNotMatch(decryptPs1, /throw "IDM start queue failed"/)
  assert.match(decryptPs1, /if \(\$null -ne \$LASTEXITCODE -and \$LASTEXITCODE -ne 0\) \{ Log \("IDM start queue returned code "/)
  assert.match(decryptPs1, /Read-Host "Wait until IDM finishes all downloads, then press Enter to finish"/)
  assert.match(decryptPs1, /while \(\$pending.Count -gt 0\)/)
})

test('task package skips existing downloads and avoids double decrypting mp4 files', () => {
  const pack = generateIdmTaskPackage({
    items: [encryptedVideoItem('secret')],
    saveDirectory: 'G:\\下载总\\res-downloader',
    batchNum: 1,
    totalBatches: 1,
    mixed: false,
  })
  const decryptPs1 = pack.files.find(file => file.path === 'decrypt.ps1').content

  assert.match(decryptPs1, /function Test-Mp4File/)
  assert.match(decryptPs1, /skip existing file/)
  assert.match(decryptPs1, /already decrypted/)
})

test('task package uses safe temporary filenames for IDM video downloads', () => {
  const pack = generateIdmTaskPackage({
    items: [
      encryptedVideoItem('secret-a'),
      {
        ...encryptedVideoItem('secret-b'),
        Description: '用这个神器打造超准AI知识库，体验AI极致编程！#cooladmin',
      },
    ],
    saveDirectory: 'G:\\下载总\\res-downloader',
    batchNum: 1,
    totalBatches: 1,
    mixed: false,
  })
  const tasks = JSON.parse(pack.files.find(file => file.path === 'idm_tasks.json').content)
  const decryptPs1 = pack.files.find(file => file.path === 'decrypt.ps1').content

  assert.equal(tasks.downloads[0].tempFilename, tasks.downloads[0].filename)
  assert.equal(tasks.downloads[1].tempFilename, tasks.downloads[1].filename)
  assert.match(tasks.downloads[1].filename, /#cooladmin\.mp4$/)
  assert.match(decryptPs1, /Move-Item -LiteralPath \$downloadFile -Destination \$targetFile -Force/)
  assert.match(decryptPs1, /\/f \$downloadName/)
})

test('task package stages videos that do not need decrypting before moving to final filenames', () => {
  const pack = generateIdmTaskPackage({
    items: [videoItem('plain')],
    saveDirectory: 'G:\\下载总\\res-downloader',
    batchNum: 1,
    totalBatches: 1,
    mixed: false,
  })
  const tasks = JSON.parse(pack.files.find(file => file.path === 'idm_tasks.json').content)

  assert.equal(tasks.downloads[0].downloadDirectory, 'idm_downloads')
  assert.equal(tasks.downloads[0].tempFilename, tasks.downloads[0].filename)
  assert.equal(tasks.downloads[0].filename, 'video plain.mp4')
  assert.equal(tasks.downloads[0].encrypted, false)
})

test('task package applies WeChat quality rules used by direct download', () => {
  const pack = generateIdmTaskPackage({
    items: [wechatVideoItem('quality')],
    saveDirectory: 'G:\\下载总\\res-downloader',
    batchNum: 1,
    totalBatches: 1,
    mixed: false,
    quality: 2,
  })
  const tasks = JSON.parse(pack.files.find(file => file.path === 'idm_tasks.json').content)

  assert.equal(tasks.downloads[0].originalUrl, wechatVideoItem('quality').Url)
  assert.match(tasks.downloads[0].url, /X-snsvideoflag=low-format/)
})

test('mixed task package stores image set metadata and keeps images out of IDM', () => {
  const pack = generateIdmTaskPackage({
    items: [imageSetItem('pkg', 2), videoItem('root')],
    saveDirectory: 'G:\\下载总\\res-downloader',
    batchNum: 1,
    totalBatches: 1,
    mixed: true,
  })
  const tasks = JSON.parse(pack.files.find(file => file.path === 'idm_tasks.json').content)

  assert.equal(tasks.downloads.length, 1)
  assert.equal(tasks.imageSets.length, 1)
  assert.equal(tasks.imageSets[0].images.length, 2)
  assert.match(tasks.imageSets[0].metadataPath, /metadata\.json$/)
  assert.equal(tasks.downloads[0].filename, 'video root.mp4')
})

test('task package sends all IDM downloads to a short staging directory', () => {
  const pack = generateIdmTaskPackage({
    items: [encryptedVideoItem('secret'), imageSetItem('pkg', 2)],
    saveDirectory: 'G:\\下载总\\res-downloader',
    batchNum: 1,
    totalBatches: 1,
    mixed: true,
  })
  const tasks = JSON.parse(pack.files.find(file => file.path === 'idm_tasks.json').content)
  const decryptPs1 = pack.files.find(file => file.path === 'decrypt.ps1').content

  assert.equal(tasks.downloads.length, 1)
  assert.equal(new Set(tasks.downloads.map(item => item.downloadDirectory)).size, 1)
  assert.equal(tasks.downloads[0].downloadDirectory, 'idm_downloads')
  assert.equal(tasks.imageSets[0].images[0].filename, '001.jpg')
  assert.equal(tasks.imageSets[0].images[1].filename, '002.jpg')
  assert.match(decryptPs1, /FullPath \$task\.downloadDirectory/)
})

test('task package downloads image sets directly and only waits on IDM videos', () => {
  const pack = generateIdmTaskPackage({
    items: [imageSetItem('pkg', 2)],
    saveDirectory: 'G:\\下载总\\res-downloader',
    batchNum: 1,
    totalBatches: 1,
    mixed: true,
  })
  const tasks = JSON.parse(pack.files.find(file => file.path === 'idm_tasks.json').content)
  const decryptPs1 = pack.files.find(file => file.path === 'decrypt.ps1').content

  assert.equal(tasks.downloads.length, 0)
  assert.equal(tasks.imageSets[0].images.length, 2)
  assert.match(decryptPs1, /function Download-ImageSets/)
  assert.match(decryptPs1, /Invoke-WebRequest -Uri \$url/)
  assert.match(decryptPs1, /if \(\(@\(\$data\.downloads\)\)\.Count -gt 0\)/)
  assert.match(decryptPs1, /if \(\(@\(\$data\.imageSets\)\)\.Count -gt 0\)/)
  assert.match(decryptPs1, /Log "no IDM video tasks"/)
  assert.match(decryptPs1, /function Resolve-Download/)
  assert.match(decryptPs1, /function Test-StableFile/)
  assert.match(decryptPs1, /Move-Item -LiteralPath \$downloadFile -Destination \$targetFile -Force/)
  assert.match(decryptPs1, /while \(\$pending.Count -gt 0\)/)
  assert.match(decryptPs1, /\$pending = New-Object System\.Collections\.Generic\.List\[object\]/)
  assert.match(decryptPs1, /foreach \(\$task in \$next\) \{ \$pending\.Add\(\$task\) \| Out-Null \}/)
  assert.match(decryptPs1, /\$deadline = \(Get-Date\)\.AddMinutes\(30\)/)
  assert.match(decryptPs1, /warning missing or unfinished files/)
})

test('task package carries image set audio into metadata and direct downloads', () => {
  const pack = generateIdmTaskPackage({
    items: [imageSetWithAudioItem('audio', 2)],
    saveDirectory: 'G:\\下载总\\res-downloader',
    batchNum: 1,
    totalBatches: 1,
    mixed: true,
  })
  const tasks = JSON.parse(pack.files.find(file => file.path === 'idm_tasks.json').content)
  const decryptPs1 = pack.files.find(file => file.path === 'decrypt.ps1').content

  assert.equal(tasks.imageSets[0].audio.url, 'https://wx.example.com/bgm.m4a')
  assert.equal(tasks.imageSets[0].audio.fileName, 'bgm.m4a')
  assert.match(decryptPs1, /function Download-ImageSets/)
  assert.match(decryptPs1, /audio/)
  assert.match(decryptPs1, /Write-ImageSetMetadata/)
})

test('image sets with the same title still use different save targets', () => {
  const first = imageSetItem('same-a', 2)
  const second = imageSetItem('same-b', 2)
  first.Description = 'same title'
  second.Description = 'same title'

  const bat = generateIdmMixedBat({
    items: [first, second],
    saveDirectory: 'G:\\下载总\\res-downloader',
    batchNum: 1,
    totalBatches: 1,
  })
  const setDirs = [...bat.matchAll(/set "SET_DIR=%BASE_DIR%\\([^"]+)"/g)].map(match => match[1])
  const imgDirs = [...bat.matchAll(/set "IMG_DIR=%BASE_DIR%\\([^"]+\\images)"/g)].map(match => match[1])
  const blocks = bat.split(/set "SET_DIR=/).slice(1)
  const targets = blocks.flatMap(block => {
    const dir = block.match(/set "IMG_DIR=%BASE_DIR%\\([^"]+)"/)?.[1]
    return [...block.matchAll(/\/p "%IMG_DIR%" \/f "([^"]+)"/g)]
        .map(match => `${dir}\\${match[1]}`)
  })

  assert.equal(setDirs.length, 2)
  assert.equal(new Set(setDirs).size, 2)
  assert.equal(imgDirs.length, 2)
  assert.equal(new Set(imgDirs).size, 2)
  assert.equal(targets.length, 4)
  assert.equal(new Set(targets).size, targets.length)
})

test('each image set writes metadata through its explicit json path and verifies it exists', () => {
  const bat = generateIdmMixedBat({
    items: [imageSetItem('meta-a', 1), imageSetItem('meta-b', 1)],
    saveDirectory: 'G:\\下载总\\res-downloader',
    batchNum: 1,
    totalBatches: 1,
  })

  const metaTargets = [...bat.matchAll(/set "META_JSON=%BASE_DIR%\\([^"]+\\metadata\.json)"/g)]
      .map(match => match[1])

  assert.equal(metaTargets.length, 2)
  assert.equal(new Set(metaTargets).size, 2)
  assert.match(bat, /WriteAllText\(\$env:META_JSON,/)
  assert.doesNotMatch(bat, /Join-Path \$env:SET_DIR 'metadata\.json'/)
  assert.equal((bat.match(/if not exist "%META_JSON%"/g) || []).length, 2)
})
