import type { InputHTMLAttributes, SelectHTMLAttributes } from 'react'
import { cn } from '../../lib/utils'

export function Input({ className, ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return (
    <input
      className={cn(
        'h-9 w-full rounded-md border border-[#cbd5d9] bg-white px-3 text-sm text-[#172026] outline-none transition placeholder:text-[#82939a] focus:border-[#0f766e] focus:ring-2 focus:ring-[#99f6e4]',
        className,
      )}
      {...props}
    />
  )
}

export function Select({ className, ...props }: SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <select
      className={cn(
        'h-9 w-full rounded-md border border-[#cbd5d9] bg-white px-3 text-sm text-[#172026] outline-none transition focus:border-[#0f766e] focus:ring-2 focus:ring-[#99f6e4]',
        className,
      )}
      {...props}
    />
  )
}
