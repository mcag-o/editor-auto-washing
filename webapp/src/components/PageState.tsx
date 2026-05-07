import CircularProgress from '@mui/material/CircularProgress';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';

type PageStateProps = {
  title: string;
  description: string;
  tone: 'loading' | 'empty' | 'error';
};

export default function PageState({ title, description, tone }: PageStateProps) {
  return (
    <Stack
      spacing={1}
      alignItems="center"
      justifyContent="center"
      role={tone === 'error' ? 'alert' : 'status'}
      data-testid={`page-state-${tone}`}
      sx={{
        minHeight: 180,
        px: 2,
        py: 5,
        textAlign: 'center',
        borderRadius: 3,
        border: '1px dashed',
        borderColor: tone === 'error' ? 'error.light' : 'divider',
        bgcolor: tone === 'error' ? 'rgba(211, 47, 47, 0.04)' : 'background.default',
      }}
    >
      {tone === 'loading' ? <CircularProgress size={24} /> : null}
      <Typography variant="subtitle2">{title}</Typography>
      <Typography variant="body2" color={tone === 'error' ? 'error.main' : 'text.secondary'}>
        {description}
      </Typography>
    </Stack>
  );
}
