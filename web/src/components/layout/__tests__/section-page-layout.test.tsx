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
import { describe, expect, test } from 'vitest'

import { Button } from '@/components/ui/button'

import { PageFooterPortal } from '../components/page-footer'
import { SectionPageLayout } from '../components/section-page-layout'

describe('panel page layout', () => {
  test('a long title truncates while page actions can wrap on narrow screens', () => {
    render(
      <SectionPageLayout stackActionsOnMobile>
        <SectionPageLayout.Title>
          A long workspace title that must leave room for page actions
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button>Filter results</Button>
        </SectionPageLayout.Actions>
      </SectionPageLayout>
    )

    const heading = screen.getByRole('heading', { level: 2 })
    expect(heading).toHaveClass('truncate')
    expect(heading.parentElement).toHaveClass('min-w-0', 'max-sm:basis-full')
    expect(screen.getByRole('button').parentElement).toHaveClass(
      'flex-wrap',
      'max-w-full'
    )
  })

  test.each([
    { fixedContent: false, overflow: 'overflow-auto' },
    { fixedContent: true, overflow: 'overflow-hidden' },
  ])(
    'fixedContent=$fixedContent keeps its $overflow content separate from the page footer',
    (props) => {
      render(
        <SectionPageLayout fixedContent={props.fixedContent}>
          <SectionPageLayout.Title>Usage</SectionPageLayout.Title>
          <SectionPageLayout.Content>
            <p>Page records</p>
            <PageFooterPortal>
              <Button>Next page</Button>
            </PageFooterPortal>
          </SectionPageLayout.Content>
        </SectionPageLayout>
      )

      const content = screen.getByText('Page records').parentElement
      const footer = screen.getByRole('button', {
        name: 'Next page',
      }).parentElement
      expect(content).toHaveClass('min-h-0', 'flex-1', props.overflow)
      expect(footer).toHaveClass('shrink-0', 'border-t')
      expect(content?.contains(footer)).toBe(false)
    }
  )
})
