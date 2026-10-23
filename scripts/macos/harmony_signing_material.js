// Hvigor 6.24.5 reads encrypted signing passwords from build-profile.json5.
// Create its fd/ac/ce material in the disposable project beside the copied keystore.
'use strict';
const crypto = require('crypto');
const fs = require('fs');
const path = require('path');

const directory = process.argv[2];
const [storePassword, keyPassword] = JSON.parse(fs.readFileSync(0, 'utf8'));
const material = path.join(directory, 'material');
fs.mkdirSync(material, { mode: 0o700 });

function seal(key, plaintext) {
  const iv = crypto.randomBytes(12);
  const cipher = crypto.createCipheriv('aes-128-gcm', key, iv);
  const encrypted = Buffer.concat([cipher.update(plaintext), cipher.final()]);
  const size = Buffer.alloc(4);
  size.writeUInt32BE(encrypted.length + 16);
  return Buffer.concat([size, iv, encrypted, cipher.getAuthTag()]);
}

function writePart(name, data) {
  const parent = path.join(material, name);
  fs.mkdirSync(parent, { mode: 0o700 });
  fs.writeFileSync(path.join(parent, 'data'), data, { mode: 0o600, flag: 'wx' });
}

const parts = [crypto.randomBytes(16), crypto.randomBytes(16), crypto.randomBytes(16)];
const salt = crypto.randomBytes(16);
const component = new Int8Array([49, 243, 9, 115, 214, 175, 91, 184, 211, 190, 177, 88, 101, 131, 192, 119]);
const mixed = new Int8Array(16);
for (let index = 0; index < 16; index++) {
  mixed[index] = parts[0][index] ^ parts[1][index] ^ parts[2][index] ^ component[index];
}
const root = crypto.pbkdf2Sync(Buffer.from(mixed).toString(), salt, 10000, 16, 'sha256');
const work = crypto.randomBytes(16);
fs.mkdirSync(path.join(material, 'fd'), { mode: 0o700 });
for (let index = 0; index < parts.length; index++) {
  const parent = path.join(material, 'fd', String(index));
  fs.mkdirSync(parent, { mode: 0o700 });
  fs.writeFileSync(path.join(parent, 'data'), parts[index], { mode: 0o600, flag: 'wx' });
}
writePart('ac', salt);
writePart('ce', seal(root, work));

process.stdout.write(JSON.stringify([
  seal(work, Buffer.from(storePassword, 'utf8')).toString('hex'),
  seal(work, Buffer.from(keyPassword, 'utf8')).toString('hex')
]));
