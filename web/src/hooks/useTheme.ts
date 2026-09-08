import { createContext, useContext } from 'react';
export type Theme = 'dark' | 'light' | 'system';
export const ThemeContext = createContext<{theme: Theme;setTheme:(theme:Theme)=>void}>({theme:'dark',setTheme:()=>{}});
export function useTheme(){return useContext(ThemeContext);}
