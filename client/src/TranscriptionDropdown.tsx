import { DownOutlined } from "@ant-design/icons";
import { Button, Dropdown, Menu } from "antd";
import * as React from "react";

export const TranscriptionDropdown = ({
  transcriptions,
  onSelect,
}: {
  transcriptions: string[];
  onSelect: (transcription: string) => void;
}) => {
  const formatPreview = (transcription: string) => {
    if (!transcription) return "N/A";
    const first20 = transcription.slice(0, 20);
    const last20 = transcription.slice(-20);
    if (first20.length + last20.length > transcription.length) {
      return transcription;
    }
    return `${first20}...${last20}`;
  };

  const menu = (
    <Menu>
      {transcriptions.map((transcription, index) => (
        <Menu.Item key={index} onClick={() => onSelect(transcription)}>
          {formatPreview(transcription)}
        </Menu.Item>
      ))}
    </Menu>
  );

  return (
    <Dropdown overlay={menu}>
      <Button>
        Choose Transcription <DownOutlined />
      </Button>
    </Dropdown>
  );
};
