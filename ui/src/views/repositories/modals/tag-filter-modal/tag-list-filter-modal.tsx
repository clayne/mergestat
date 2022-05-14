import React from 'react'
import { Button, Input, Modal, Toolbar, Checkbox } from '@mergestat/blocks'
import { SearchIcon, XIcon } from '@mergestat/icons'
import { ModalFooter } from './modal-footer'


type TagListFilterModalType = {
  open: boolean
  searchText?: string
  tags: { title: string, checked: boolean }[]
  handleCheck: (checked: boolean, index: number) => void
  handleSearch: (text: string) => void
  onClose: () => void
}

export const TagListFilterModal: React.FC<TagListFilterModalType> = (props) => {
  const { open, tags, handleCheck, handleSearch, onClose, searchText } = props
  return (
    <Modal open={open} onClose={onClose} size="lg" className="max-w-6xl">
      <Modal.Header>
        <Toolbar className="h-16 px-6">
          <Toolbar.Left>
            <Toolbar.Item>
              <Modal.Title>Filter</Modal.Title>
            </Toolbar.Item>
          </Toolbar.Left>
          <Toolbar.Right>
            <Input
              placeholder="Search..."
              startIcon={<SearchIcon className="t-icon text-gray-400" />}
              value={searchText}
              onChange={(e: any) => handleSearch(e.target.value)}
            />
            <Toolbar.Item>
              <Button
                skin="borderless-muted"
                onClick={onClose}
                startIcon={<XIcon className="t-icon" />}
              />
            </Toolbar.Item>
          </Toolbar.Right>
        </Toolbar>
      </Modal.Header>
      <Modal.Body>
        <div className='px-6 py-8 h-96 flex flex-col flex-wrap overflow-x-auto gap-y-5 gap-x-16'>
          {tags.length === 0 &&
            <p className='m-auto'>
              Empty!
            </p>
          }
          {tags.map((tag, index) => (
            <div key={index} className=" flex items-center gap-x-3 text-sm">
              <Checkbox
                checked={tag.checked}
                onClick={() => handleCheck(tag.checked, index)}
                onChange={(e) => {
                  const checked = e.currentTarget.checked
                }}
              />
              <span>{tag.title}</span>
            </div>
          ))}
        </div>
      </Modal.Body>
      <ModalFooter onClose={onClose} />
    </Modal >
  )
}
