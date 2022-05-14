import { Table } from '@mergestat/blocks'
import React from 'react'
import { getRepositorySyncIcon } from 'src/views/repository-data-details'
import { columns } from './columns'
import {
  RepositoryData,
  RepositorySyncStatus,
  RepositorySyncNow,
} from './repository-data-logs-table-columns'
import { RepositoryTableRowOptions } from './repository-table-row-options'
import { sampleRepositoryData } from './sample-data'

export const RepositoryTable: React.FC = (props) => {
  // TODO: export this logic to a hook
  const processedData = sampleRepositoryData.map((item, index) => ({
    syncStateIcon: item.syncNow.syncState,
    data: {
      title: item.Data.title,
      brief: item.Data.brief,
    },
    latest_run: {
      time_ago: item.latest_run,
      disabled: item.syncNow.syncState === 'disabled'
    },
    status:{
      graphNode:item.status.graphNode,
      disabled:item.syncNow.syncState === 'disabled'
    },
    syncNow:{
      syncStatus:item.syncNow.syncState,
      doSync:item.syncNow.doSync
    } ,
    options:{
     state:item.syncNow.syncState
    },
  }))
  return (
    <div className="rounded-md">
      <Table
        scrollY={'100%'}
        noWrapHeaders
        tableWrapperClassName="overflow-visible overflow-y-visible"
        className="relative z-0"
        columns={columns}
        dataSource={processedData}
      />
    </div>
  )
}

