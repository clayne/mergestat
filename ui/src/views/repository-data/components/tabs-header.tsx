import { Tabs } from '@mergestat/blocks'
import React from 'react'

export const TabsHeader: React.FC = (props) => {
  return (
    <div>
      <Tabs.List className="bg-white w-full justify-between px-8 items-center border-b border-gray-200">
        <Tabs.Item className="ring-transparent focus_ring-transparent">
          Sync Types
        </Tabs.Item>
        <Tabs.Item>Repo Settings</Tabs.Item>
      </Tabs.List>
    </div>
  )
}