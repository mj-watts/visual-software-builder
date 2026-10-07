import type { GroupColour } from './types'
export const groupPalette = [
 {name:'orange',hex:'#ff7043'}, {name:'blue',hex:'#2684ff'},
 {name:'green',hex:'#43d11e'}, {name:'amber',hex:'#ffc400'}, {name:'plum',hex:'#a855f7'},
] as const
export function groupHex(color:GroupColour) {
 return groupPalette.find(choice=>choice.name===color)?.hex ?? color
}
function linearChannel(channel:number) {
 const value=channel/255
 return value<=0.04045 ? value/12.92 : ((value+0.055)/1.055)**2.4
}
export function groupAppearance(color:GroupColour) {
 const hex=groupHex(color)
 const channels=[1,3,5].map(index=>linearChannel(parseInt(hex.slice(index,index+2),16)))
 const luminance=channels[0]!*0.2126+channels[1]!*0.7152+channels[2]!*0.0722
 const darkInk=luminance>0.179
 return {'--group-color':hex,'--group-ink':darkInk?'#000000':'#ffffff','--group-control':darkInk?'#ffffff55':'#00000033'}
}
