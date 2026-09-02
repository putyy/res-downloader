import test from 'node:test'
import assert from 'node:assert/strict'

import {isWechatChannelResource} from '../src/func.ts'

test('recognizes current WeChat 4.1.11 finder CDN resources', () => {
    assert.equal(isWechatChannelResource({
        Domain: 'qq.com:443',
        Url: 'https://findera4.video.qq.com:443/251/20302/stodownload'
    }), true)
})

test('keeps compatibility with legacy finder resource hosts', () => {
    assert.equal(isWechatChannelResource({
        Domain: 'qq.com',
        Url: 'https://finder.video.qq.com/251/20302/stodownload'
    }), true)
    assert.equal(isWechatChannelResource({
        OtherData: {wx_channel: '1'}
    }), true)
})

test('does not expose comment actions for unrelated resources', () => {
    assert.equal(isWechatChannelResource({
        Domain: 'qq.com:443',
        Url: 'https://video.qq.com/movie.mp4'
    }), false)
    assert.equal(isWechatChannelResource({
        Domain: 'example.com',
        Url: 'https://finder.video.qq.com.example.com/movie.mp4'
    }), false)
    assert.equal(isWechatChannelResource({
        Domain: 'qq.com',
        Url: 'not a url'
    }), false)
})
