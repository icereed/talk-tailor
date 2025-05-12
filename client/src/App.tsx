import { DownOutlined } from "@ant-design/icons";
import {
  Alert,
  Button,
  Col,
  ColProps,
  Dropdown,
  Input,
  Menu,
  Row,
  Space,
  Spin,
  Typography,
  Upload,
} from "antd";
import "antd/dist/reset.css";
import { openDB } from "idb";
import * as React from "react";
import { useEffect, useRef, useState } from "react";
import { FiCopy, FiMic, FiMicOff, FiUpload } from "react-icons/fi";
import "webrtc-adapter";
import { TranscriptionDropdown } from "./TranscriptionDropdown";
import {
  convertVideoToMP3,
  convertWAVtoMP3,
  sendMP3ToBackend,
} from "./audioUtils";

const { TextArea } = Input;
const { Title, Paragraph } = Typography;

async function storeMP3(mp3URL: string, mp3Blob: Blob) {
  const db = await openDB("TalkTailor", 1, {
    upgrade(db) {
      db.createObjectStore("mp3");
    },
  });

  const tx = db.transaction("mp3", "readwrite");
  const store = tx.objectStore("mp3");
  await store.put(mp3Blob, mp3URL);
  await tx.done;
}

async function getMP3(mp3URL: string) {
  const db = await openDB("TalkTailor", 1);
  const mp3Blob = await db.get("mp3", mp3URL);
  return mp3Blob;
}

const AIApplicationDropdown: React.FC<{
  onGenerateOutline: () => void;
  onConvertToBulletpoints: () => void;
}> = ({ onGenerateOutline, onConvertToBulletpoints }) => {
  const menu = (
    <Menu>
      <Menu.Item key="1" onClick={onGenerateOutline}>
        Generate Outline
      </Menu.Item>
      <Menu.Item key="2" onClick={onConvertToBulletpoints}>
        Convert to Bulletpoints
      </Menu.Item>
    </Menu>
  );

  return (
    <Dropdown overlay={menu}>
      <Button type="primary" size="large">
        Use AI <DownOutlined />
      </Button>
    </Dropdown>
  );
};

const App: React.FC = () => {
  const recordButton = useRef<HTMLButtonElement>(null);
  const stopButton = useRef<HTMLButtonElement>(null);
  const [error, setError] = useState<string | null>(null);
  const audioPlayer = useRef<HTMLAudioElement>(null);
  const [audioURL, setAudioURL] = useState<string | null>(null);
  const transcriptionElement = useRef<HTMLTextAreaElement>(null);
  const [isEditing, setIsEditing] = useState(false);
  const [isRecording, setIsRecording] = useState(false);
  const defaultTranscriptionContent =
    "Start a new recording to show some transcription or choose a past recording from the button below.";
  const [transcription, setTranscription] = useState<string | null>(
    defaultTranscriptionContent
  );

  const storedTranscriptions = JSON.parse(
    localStorage.getItem("transcriptions") || "[]"
  );

  const [transcriptions, setTranscriptions] = useState<
    Array<{ transcription: string; mp3URL: string }>
  >(storedTranscriptions.length > 0 ? storedTranscriptions : []);

  useEffect(() => {
    localStorage.setItem("transcriptions", JSON.stringify(transcriptions));
  }, [transcriptions]);

  const [isLoading, setIsLoading] = useState(false);

  const mediaRecorder = useRef<MediaRecorder | null>(null);

  const addTranscription = async (
    newTranscription: string,
    mp3URL: string,
    mp3Blob?: Blob
  ) => {
    setTranscription(newTranscription);
    setTranscriptions((prevTranscriptions) => {
      const deduplicatedTranscriptions = [
        { transcription: newTranscription, mp3URL },
        ...prevTranscriptions,
      ]
        .filter(
          // Deduplicate
          (item, index, self) =>
            self.findIndex((t) => t.transcription === item.transcription) ===
            index
        )
        .filter(
          // Filter out those with empty transcription
          (item) => item.transcription !== ""
        );
      return deduplicatedTranscriptions.slice(0, 10);
    });

    if (mp3Blob) {
      await storeMP3(mp3URL, mp3Blob);
    }
  };

  const handleUploadFile = async (file: File) => {
    setIsLoading(true);
    setError(null);

    try {
      let mp3Blob: Blob;

      if (file.type === "audio/mpeg") {
        mp3Blob = new Blob([file], { type: "audio/mpeg" });
      } else {
        mp3Blob = await convertVideoToMP3(mp3Blob);
      }

      setAudioURL(URL.createObjectURL(mp3Blob));
      addTranscription(
        await sendMP3ToBackend(mp3Blob),
        URL.createObjectURL(mp3Blob),
        mp3Blob
      );
    } catch (error) {
      setError(
        "There was an error transcribing your file. Please try again later."
      );
    } finally {
      setIsLoading(false);
    }
  };

  const handleStartRecording = async () => {
    console.log("start recording");

    console.log("start media recording");
    const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
    mediaRecorder.current = new MediaRecorder(stream);
    let audioChunks: Blob[] = [];

    mediaRecorder.current.addEventListener("dataavailable", (event) => {
      console.log("data available");
      audioChunks.push(event.data);
    });

    mediaRecorder.current.addEventListener("stop", async () => {
      console.log("stop");
      setIsLoading(true);
      const blob = new Blob(audioChunks, { type: "audio/wav" });

      // Log the WAV URL to listen to the original recording
      console.log("WAV URL:", URL.createObjectURL(blob));
      const mp3Blob = await convertWAVtoMP3(blob);
      const url = URL.createObjectURL(mp3Blob);
      setAudioURL(url);
      try {
        addTranscription(
          await sendMP3ToBackend(mp3Blob),
          URL.createObjectURL(mp3Blob),
          mp3Blob
        );
      } catch (error) {
        setError(
          "There was an error transcribing your recording. Please try again later."
        );
      } finally {
        setIsLoading(false);
      }
      audioChunks = [];
    });

    console.log("mediaRecorder.start()");
    mediaRecorder.current.start();
    setIsRecording(true);
  };

  const handleStopRecording = () => {
    console.log("stop recording");
    console.log(mediaRecorder.current);
    if (mediaRecorder.current) {
      mediaRecorder.current.stop();
      mediaRecorder.current.stream.getTracks().forEach((track) => track.stop());
    }
    setIsRecording(false);
  };

  const handleRetry = async () => {
    setError(null);
    // get MP3 blob from audioURL
    const mp3Blob = await fetch(audioURL || "").then((res) => res.blob());
    setIsLoading(true);
    try {
      addTranscription(
        await sendMP3ToBackend(mp3Blob),
        URL.createObjectURL(mp3Blob),
        mp3Blob
      );
    } catch (error) {
      setError(
        "There was an error transcribing your recording. Please try again later."
      );
    } finally {
      setIsLoading(false);
    }
  };

  const copyTranscriptionToClipboard = async () => {
    if (transcription) {
      await navigator.clipboard.writeText(transcription);
      alert("Transcription copied to clipboard");
    }
  };

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      switch (event.key) {
        case "r":
          if (!isRecording && !isEditing) {
            handleStartRecording();
          }
          break;
        case "s":
          if (isRecording) {
            handleStopRecording();
          }
          break;
        case "c":
          if (!isEditing) {
            // Add this condition
            copyTranscriptionToClipboard();
          }
          break;
        default:
          break;
      }
    };

    window.addEventListener("keydown", handleKeyDown);
    return () => {
      window.removeEventListener("keydown", handleKeyDown);
    };
  }, [transcription, isEditing, isRecording]);

  const [aiResult, setAIResult] = useState<string | null>(null);
  const [isProcessingAIAction, setIsProcessingAIAction] = useState(false);

  const sendTranscriptionToOutlineAPI = async (transcription: string) => {
    setIsProcessingAIAction(true);
    const requestOptions = {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      // Post form data - use key "text" for the transcription
      body: JSON.stringify({ text: transcription }),
    };

    const response = await fetch("/api/outline", requestOptions);
    const data = await response.json();
    setAIResult(data.response);
    setIsProcessingAIAction(false);
  };

  const handleConvertToBulletpoints = async (transcription: string) => {
    setIsProcessingAIAction(true);
    console.log("Send transcription to convert to bulletpoints API");
    const requestOptions = {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      // Post form data - use key "text" for the transcription
      body: JSON.stringify({ text: transcription }),
    };

    const response = await fetch("/api/bulletpoints", requestOptions);
    const data = await response.json();
    setAIResult(data.response);
    setIsProcessingAIAction(false);
  };

  const renderOutline = () => {
    if (!aiResult) {
      return <p />;
    }

    return (
      <div>
        <h3>Outline</h3>
        <pre
          style={{
            whiteSpace: "pre-wrap",
            wordWrap: "break-word",
            overflowWrap: "break-word",
          }}
        >
          {aiResult}
        </pre>
      </div>
    );
  };

  // extract common grid styles
  const gridStyle: ColProps = {
    xs: { span: 24 },
    sm: { span: 16, offset: 4 },
    //md: { span: 12, offset: 6 },
  };

  return (
    <div style={{ padding: "1rem" }}>
      <Row>
        <Col {...gridStyle}>
          <Title level={2} style={{ textAlign: "center" }}>
            TalkTailor
          </Title>
          <Paragraph
            style={{
              textAlign: "center",
              fontSize: "1.125rem",
              marginBottom: "2rem",
            }}
          >
            TalkTailor helps you master the art of public speaking by
            transcribing your talks, providing feedback, and helping you improve
            your speech. To get started, record your talk, and let our AI
            transcribe and analyze it for you.
          </Paragraph>
        </Col>
      </Row>

      <Row>
        <Col {...gridStyle}>
          <Space
            size="large"
            direction="vertical"
            style={{ width: "100%", textAlign: "center" }}
          >
            <Button
              id="recordButton"
              ref={recordButton}
              type="primary"
              size="large"
              onClick={handleStartRecording}
              title="Press 'R' to start recording"
              disabled={isRecording}
              block
            >
              <FiMic /> Start Recording
            </Button>
            <Button
              id="stopButton"
              ref={stopButton}
              type="primary"
              danger
              size="large"
              onClick={handleStopRecording}
              title="Press 'S' to stop recording"
              disabled={!isRecording}
              block
            >
              <FiMicOff /> Stop Recording
            </Button>
            <Upload.Dragger
              accept=".mp3,video/*"
              showUploadList={false}
              beforeUpload={(file) => {
                handleUploadFile(file);
                return false;
              }}
            >
              <Button
                icon={<FiUpload />}
                size="large"
                title="Upload a pre-recorded MP3 or video file"
                disabled={isLoading}
                block
              >
                Upload Recording
              </Button>
            </Upload.Dragger>
          </Space>
        </Col>
      </Row>
      {error && (
        <Row style={{ marginBottom: "1rem" }}>
          <Col {...gridStyle}>
            <Alert
              message={error}
              description="There was an issue with the transcription service. Please try again."
              type="error"
              action={
                <Button size="small" type="primary" onClick={handleRetry}>
                  Retry
                </Button>
              }
              showIcon
            />
          </Col>
        </Row>
      )}
      <Row>
        <Space
          size="large"
          direction="vertical"
          style={{ width: "100%", textAlign: "center", marginTop: "3rem" }}
        >
          <Col xs={24}>
            <Spin spinning={isLoading} size="large" />
          </Col>
        </Space>
      </Row>

      <Row style={{ marginTop: "3rem" }}>
        <Col {...gridStyle}>
          <Title level={3} style={{ textAlign: "center" }}>
            Transcription
          </Title>
          <div style={{ marginTop: "1rem" }}>
            <TextArea
              id="transcription"
              ref={transcriptionElement}
              style={{
                borderRadius: "0.25rem",
                width: "100%",
                padding: "1rem",
                fontSize: "1.125rem",
                minHeight: "15rem",
              }}
              value={transcription}
              onChange={(event: React.ChangeEvent<HTMLTextAreaElement>) =>
                setTranscription(event.target.value)
              }
              onFocus={() => setIsEditing(true)}
              onBlur={() => {
                setIsEditing(false);
                addTranscription(transcription, audioURL);
              }}
            />
          </div>
          <Space
            size="large"
            style={{
              justifyContent: "center",
              marginTop: "1rem",
              width: "100%",
              textAlign: "center",
            }}
          >
            <TranscriptionDropdown
              transcriptions={transcriptions.map((t) => t.transcription)}
              onSelect={async (transcription) => {
                const selected = transcriptions.find(
                  (t) => t.transcription === transcription
                );
                if (selected) {
                  setTranscription(selected.transcription);
                  const mp3Blob = await getMP3(selected.mp3URL);
                  const newURL = URL.createObjectURL(mp3Blob);
                  setAudioURL(newURL);
                }
              }}
            />
            <Button
              type="default"
              size="large"
              onClick={copyTranscriptionToClipboard}
              title="Press to copy the transcription to clipboard"
            >
              <FiCopy /> Copy to Clipboard
            </Button>
            <AIApplicationDropdown
              onGenerateOutline={() =>
                sendTranscriptionToOutlineAPI(transcription || "")
              }
              onConvertToBulletpoints={() =>
                handleConvertToBulletpoints(transcription || "")
              }
            />
            <Spin spinning={isProcessingAIAction} />
          </Space>
          <audio
            id="audioPlayer"
            ref={audioPlayer}
            src={audioURL}
            className={`${audioURL ? "" : "d-none"}`}
            controls
            style={{ width: "100%", marginTop: "1rem" }}
          />
        </Col>
      </Row>

      <Row style={{ marginTop: "3rem" }}>
        <Col {...gridStyle}>
          <div>{renderOutline()}</div>
        </Col>
      </Row>
    </div>
  );
};

export default App;
