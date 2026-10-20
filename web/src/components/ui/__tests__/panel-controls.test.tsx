/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test, vi } from 'vitest'

import { Button } from '../button'
import { Input } from '../input'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '../tabs'

describe('panel control styling contracts', () => {
  test('default form controls share a height and remain keyboard operable', async () => {
    const onSave = vi.fn()
    const user = userEvent.setup()
    render(
      <>
        <Input aria-label='Workspace name' />
        <Button onClick={onSave}>Save</Button>
      </>
    )

    const input = screen.getByRole('textbox', { name: 'Workspace name' })
    const button = screen.getByRole('button', { name: 'Save' })
    expect(input).toHaveClass('h-9')
    expect(button).toHaveClass('h-9')
    await user.tab()
    expect(input).toHaveFocus()
    await user.keyboard('Workspace')
    expect(input).toHaveValue('Workspace')
    await user.tab()
    expect(button).toHaveFocus()
    await user.keyboard('{Enter}')
    expect(onSave).toHaveBeenCalledOnce()
  })

  test('disabled buttons cannot activate and link buttons keep their destination', async () => {
    const onSave = vi.fn()
    const user = userEvent.setup()
    render(
      <>
        <Button disabled onClick={onSave}>
          Save
        </Button>
        <Button render={<a href='/dashboard' />}>Dashboard</Button>
      </>
    )

    const disabled = screen.getByRole('button', { name: 'Save' })
    expect(disabled).toBeDisabled()
    await user.click(disabled)
    expect(onSave).not.toHaveBeenCalled()
    await user.tab()
    const destination = screen.getByRole('button', { name: 'Dashboard' })
    expect(destination).toHaveAttribute('href', '/dashboard')
    expect(destination).toHaveFocus()
  })

  test('keyboard tab selection keeps the visible panel and selected state aligned', async () => {
    const user = userEvent.setup()
    render(
      <Tabs defaultValue='overview'>
        <TabsList aria-label='Workspace sections'>
          <TabsTrigger value='overview'>Overview</TabsTrigger>
          <TabsTrigger value='models'>Models</TabsTrigger>
          <TabsTrigger value='unavailable' disabled>
            Unavailable
          </TabsTrigger>
        </TabsList>
        <TabsContent value='overview'>Overview records</TabsContent>
        <TabsContent value='models'>Model records</TabsContent>
      </Tabs>
    )

    await user.tab()
    await user.keyboard('{ArrowRight}')
    expect(screen.getByRole('tab', { name: 'Models' })).toHaveFocus()
    await user.keyboard('{Enter}')
    expect(screen.getByRole('tab', { name: 'Models' })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    expect(screen.getByRole('tabpanel')).toHaveTextContent('Model records')
    expect(screen.getByRole('tab', { name: 'Unavailable' })).toHaveAttribute(
      'aria-disabled',
      'true'
    )
  })
})
