// POLY_1271 (signature type 3) ERC-7739 wrap signature reference vector generator.
// Mirrors go-clob-client signOrderV2Poly1271: innerSig(65) || appDomainSep(32) ||
// contentsHash(32) || ORDER_TYPE_STRING || uint16BE(len(ORDER_TYPE_STRING)).
// Usage: node poly1271.mjs [depositWalletAddress]
import { privateKeyToAccount } from 'viem/accounts'
import { hashDomain, keccak256, encodeAbiParameters, concatHex, stringToHex, numberToHex } from 'viem'

const account = privateKeyToAccount('0xac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80')

const depositWallet = process.argv[2]
if (!depositWallet) { console.error('usage: node poly1271.mjs <depositWallet>'); process.exit(1) }
const exchange = '0xE111180000d2663C0091e4f400237545B87B996B'
const chainId = 137
const ZERO32 = `0x${'00'.repeat(32)}`

const orderTypes = [
  { name: 'salt', type: 'uint256' },
  { name: 'maker', type: 'address' },
  { name: 'signer', type: 'address' },
  { name: 'tokenId', type: 'uint256' },
  { name: 'makerAmount', type: 'uint256' },
  { name: 'takerAmount', type: 'uint256' },
  { name: 'side', type: 'uint8' },
  { name: 'signatureType', type: 'uint8' },
  { name: 'timestamp', type: 'uint256' },
  { name: 'metadata', type: 'bytes32' },
  { name: 'builder', type: 'bytes32' },
]

const order = {
  salt: 12345n,
  maker: depositWallet,
  signer: depositWallet,
  tokenId: 123n,
  makerAmount: 10520000n,
  takerAmount: 21040000n,
  side: 0,
  signatureType: 3,
  timestamp: 1700000000000n,
  metadata: ZERO32,
  builder: ZERO32,
}

const domain = { name: 'Polymarket CTF Exchange', version: '2', chainId, verifyingContract: exchange }

const innerSig = await account.signTypedData({
  domain,
  types: {
    TypedDataSign: [
      { name: 'contents', type: 'Order' },
      { name: 'name', type: 'string' },
      { name: 'version', type: 'string' },
      { name: 'chainId', type: 'uint256' },
      { name: 'verifyingContract', type: 'address' },
      { name: 'salt', type: 'bytes32' },
    ],
    Order: orderTypes,
  },
  primaryType: 'TypedDataSign',
  message: {
    contents: order,
    name: 'DepositWallet',
    version: '1',
    chainId: BigInt(chainId),
    verifyingContract: depositWallet,
    salt: ZERO32,
  },
})

const appDomainSep = hashDomain({
  domain,
  types: {
    EIP712Domain: [
      { name: 'name', type: 'string' },
      { name: 'version', type: 'string' },
      { name: 'chainId', type: 'uint256' },
      { name: 'verifyingContract', type: 'address' },
    ],
  },
})

const ORDER_TYPE_STRING = 'Order(uint256 salt,address maker,address signer,uint256 tokenId,uint256 makerAmount,uint256 takerAmount,uint8 side,uint8 signatureType,uint256 timestamp,bytes32 metadata,bytes32 builder)'
const contentsHash = keccak256(encodeAbiParameters(
  ['bytes32','uint256','address','address','uint256','uint256','uint256','uint8','uint8','uint256','bytes32','bytes32'].map(type => ({ type })),
  [keccak256(stringToHex(ORDER_TYPE_STRING)), order.salt, order.maker, order.signer, order.tokenId,
   order.makerAmount, order.takerAmount, BigInt(order.side), BigInt(order.signatureType), order.timestamp,
   order.metadata, order.builder],
))

const wrapped = concatHex([
  innerSig,
  appDomainSep,
  contentsHash,
  stringToHex(ORDER_TYPE_STRING),
  numberToHex(ORDER_TYPE_STRING.length, { size: 2 }),
])
console.log(`depositWallet: ${depositWallet}`)
console.log(`wrapped: ${wrapped}`)
console.log(`bytes: ${(wrapped.length - 2) / 2}`)
