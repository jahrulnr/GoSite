import { useEffect, useState } from 'preact/hooks';
import { openJobStream } from '../lib/sse';
import { Modal } from './Ui';

interface JobStreamModalProps {
  title: string;
  streamUrl: string;
  onClose: () => void;
}

export function JobStreamModal({ title, streamUrl, onClose }: Readonly<JobStreamModalProps>) {
  const [lines, setLines] = useState<string[]>([]);
  const [status, setStatus] = useState('running');

  useEffect(() => {
    const close = openJobStream(streamUrl, {
      onLine: (line) => setLines((current) => [...current, line]),
      onDone: (next) => setStatus(next),
      onError: (message) => {
        setStatus('error');
        setLines((current) => [...current, message]);
      },
    });
    return close;
  }, [streamUrl]);

  return (
    <Modal title={title} onClose={onClose} wide footer={<span class="dim">Status: {status}</span>}>
      <pre class="logbox">{lines.join('\n') || 'Waiting for output…'}</pre>
    </Modal>
  );
}
