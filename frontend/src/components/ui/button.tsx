import type { ButtonHTMLAttributes } from 'react'
import { cn } from '../../lib/utils'

type ButtonVariant = 'primary' | 'secondary' | 'danger' | 'ghost'

type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> & {
  variant?: ButtonVariant
}

const variants: Record<ButtonVariant, string> = {
  primary: 'bg-[#0f766e] text-white hover:bg-[#115e59]',
  secondary: 'bg-white text-[#172026] border border-[#cbd5d9] hover:bg-[#eef3f4]',
  danger: 'bg-[#b42318] text-white hover:bg-[#912018]',
  ghost: 'bg-transparent text-[#31525b] hover:bg-[#e8eef0]',
}

export function Button({ className, variant = 'primary', ...props }: ButtonProps) {
  return (
    <button
      className={cn(
        'inline-flex h-9 items-center justify-center gap-2 rounded-md px-3 text-sm font-medium transition disabled:cursor-not-allowed disabled:opacity-50',
        variants[variant],
        className,
      )}
      {...props}
    />
  )
}
