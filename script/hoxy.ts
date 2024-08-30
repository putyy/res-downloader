const fs = require('fs');
const path = require('path');
const currentDir = process.cwd();

// 移动文件
fs.copyFile(currentDir + '/override/hoxy/lib/cycle.js', currentDir + '/node_modules/hoxy/lib/cycle.js', fs.constants.COPYFILE_FICLONE, (err) => {
});