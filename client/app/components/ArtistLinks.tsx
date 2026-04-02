import React from 'react';
import { Link } from 'react-router';

type Artist = {
  id: number;
  name: string;
};

type ArtistLinksProps = {
  artists: Artist[];
};

const ArtistLinks: React.FC<ArtistLinksProps> = ({ artists }) => {
  if (!artists) return null;
  return (
    <>
      {artists.filter(artist => artist).map((artist, index) => (
        <span key={artist.id} className='color-fg-secondary'>
          <Link className="hover:text-(--color-fg-tertiary)" to={`/artist/${artist.id}`}>{artist.name}</Link>
          {index < artists.filter(a => a).length - 1 ? ', ' : ''}
        </span>
      ))}
    </>
  );
};

export default ArtistLinks;
