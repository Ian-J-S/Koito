import { replaceImage } from 'api/api';
import { useEffect } from 'react';

interface Props {
    itemType: string,
    onComplete: Function
}

export default function ImageDropHandler({ itemType, onComplete }: Props) {
  useEffect(() => {
    const getItemId = () => {
      const pathname = window.location.pathname;
      const segments = pathname.split('/');
      const filteredSegments = segments.filter(segment => segment !== '');
      return filteredSegments[filteredSegments.length - 1];
    };

    const uploadImage = async (imageFile: File) => {
      const formData = new FormData();
      formData.append('image', imageFile);
      const itemId = getItemId();
      formData.append(itemType.toLowerCase() + '_id', itemId);

      try {
        const r = await replaceImage(formData);
        if (r.status >= 200 && r.status < 300) {
          onComplete();
          console.log("Replacement image uploaded successfully");
        } else {
          const body = await r.json();
          console.log(`Upload failed: ${r.statusText} - ${body}`);
        }
      } catch (err) {
        console.log(`Upload failed: ${err}`);
      }
    };

    const handleDragOver = (e: DragEvent) => {
      console.log('dragover!!');
      e.preventDefault();
    };

    const handleDrop = async (e: DragEvent) => {
      e.preventDefault();
      if (!e.dataTransfer?.files.length) return;

      const imageFile = Array.from(e.dataTransfer.files).find(file =>
        file.type.startsWith('image/')
      );
      if (!imageFile) return;

      await uploadImage(imageFile);
    };

    window.addEventListener('dragover', handleDragOver);
    window.addEventListener('drop', handleDrop);

    return () => {
      window.removeEventListener('dragover', handleDragOver);
      window.removeEventListener('drop', handleDrop);
    };
  }, []);

  return null;
}
