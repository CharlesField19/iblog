import { inject } from 'vue'
import type { ConfirmFunction, ToastFunction } from '../../shared/contracts'

export function useToast(): ToastFunction {
  const toast = inject<ToastFunction>('toast')
  if (!toast) throw new Error('Toast provider is missing')
  return toast
}

export function useConfirm(): ConfirmFunction {
  const confirm = inject<ConfirmFunction>('confirmAction')
  if (!confirm) throw new Error('Confirmation provider is missing')
  return confirm
}
