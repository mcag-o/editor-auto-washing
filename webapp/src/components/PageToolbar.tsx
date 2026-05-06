import type { ReactNode } from 'react';
import Box from '@mui/material/Box';
import Stack from '@mui/material/Stack';
import Typography from '@mui/material/Typography';

type PageToolbarProps = {
  title: string;
  description?: string;
  leading?: ReactNode;
  actions?: ReactNode;
  filters?: ReactNode;
};

export default function PageToolbar({
  title,
  description,
  leading,
  actions,
  filters,
}: PageToolbarProps) {
  return (
    <Stack spacing={2.5}>
      <Stack
        direction={{ xs: 'column', md: 'row' }}
        spacing={2}
        justifyContent="space-between"
        alignItems={{ xs: 'flex-start', md: 'center' }}
      >
        <Stack spacing={1.25}>
          {leading}
          <Box>
            <Typography variant="h2">{title}</Typography>
            {description ? (
              <Typography variant="body1" color="text.secondary" sx={{ mt: 0.75 }}>
                {description}
              </Typography>
            ) : null}
          </Box>
        </Stack>
        {actions ? (
          <Stack direction="row" spacing={1.25} flexWrap="wrap" useFlexGap>
            {actions}
          </Stack>
        ) : null}
      </Stack>
      {filters ? (
        <Box
          sx={{
            p: 1.5,
            borderRadius: 4,
            border: '1px solid',
            borderColor: 'divider',
            bgcolor: 'background.paper',
          }}
        >
          {filters}
        </Box>
      ) : null}
    </Stack>
  );
}
