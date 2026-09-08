import { useEffect, useState } from 'react';
import { ThemeContext, type Theme } from '@/hooks/useTheme';
export function ThemeProvider({children,defaultTheme='dark',storageKey='sennet-theme'}:{children:React.ReactNode;defaultTheme?:Theme;storageKey?:string}){
 const [theme,setTheme]=useState<Theme>(()=>{const stored=localStorage.getItem(storageKey);return stored==='light'||stored==='dark'||stored==='system'?stored:defaultTheme;});
 useEffect(()=>{const media=window.matchMedia('(prefers-color-scheme: dark)');const apply=()=>{document.documentElement.classList.remove('light','dark');document.documentElement.classList.add(theme==='system'?(media.matches?'dark':'light'):theme);};apply();media.addEventListener('change',apply);return()=>media.removeEventListener('change',apply);},[theme]);
 return <ThemeContext.Provider value={{theme,setTheme:value=>{localStorage.setItem(storageKey,value);setTheme(value);}}}>{children}</ThemeContext.Provider>;
}
